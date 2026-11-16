package sweep

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/PurgeBot-net/common/job"
	"github.com/PurgeBot-net/common/log"
	"github.com/PurgeBot-net/database"
	"github.com/PurgeBot-net/sweeper/config"
)

const (
	interval = time.Hour
	pageSize = 200
)

type Sweeper struct {
	logger *zap.Logger
	db     *database.Database
	redis  *redis.Client
	client *bot.Client
	// A refused BGREWRITEAOF is owed until one succeeds: a rewrite already running forked before our DEL.
	rewritePending bool
}

func New(cfg config.Config, logger *zap.Logger, db *database.Database, redis *redis.Client) (*Sweeper, error) {
	client, err := disgo.New(cfg.Token,
		bot.WithCacheConfigOpts(cache.WithCaches(cache.FlagsNone)),
		bot.WithLogger(log.Slog(logger)),
	)
	if err != nil {
		return nil, err
	}
	return &Sweeper{logger: logger, db: db, redis: redis, client: client}, nil
}

func (s *Sweeper) Run(ctx context.Context) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := s.Sweep(ctx); err != nil && ctx.Err() == nil {
			s.logger.Error("sweep failed", zap.Error(err))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Sweeper) Sweep(ctx context.Context) error {
	now := time.Now().UTC()

	listed, listErr := listGuildIDs(func(after int64) ([]int64, error) {
		// shard 0 lists every guild below large bot sharding (~150k guilds); past that, loop 0..max_concurrency-1.
		guilds, err := s.client.Rest.GetCurrentUserGuilds("", 0, snowflake.ID(after), pageSize, 0, false, rest.WithCtx(ctx))
		if err != nil {
			return nil, err
		}
		ids := make([]int64, len(guilds))
		for i, g := range guilds {
			ids[i] = int64(g.ID)
		}
		return ids, nil
	})

	departed, err := s.db.GetDepartures(ctx)
	if err != nil {
		return fmt.Errorf("get departures: %w", err)
	}
	// Before the listing checks: a listed guild is present even if the list broke off later.
	back := rejoined(listed, departed)
	if len(back) > 0 {
		if err := s.db.ClearDepartures(ctx, back); err != nil {
			return fmt.Errorf("clear departures: %w", err)
		}
	}
	if listErr != nil {
		return fmt.Errorf("list guilds: %w", listErr)
	}

	app, err := s.client.Rest.GetCurrentApplication(rest.WithCtx(ctx))
	if err != nil {
		return fmt.Errorf("get application: %w", err)
	}
	approx := -1
	if app.ApproximateGuildCount != nil {
		approx = *app.ApproximateGuildCount
	}
	if !listingComplete(len(listed), app.ApproximateGuildCount) {
		return fmt.Errorf("guild list looks incomplete: %d listed, approximate guild count %d", len(listed), approx)
	}

	stored, err := s.db.GuildIDsWithData(ctx)
	if err != nil {
		return fmt.Errorf("get guilds with data: %w", err)
	}
	withJobs, err := job.GuildIDs(ctx, s.redis)
	if err != nil {
		return fmt.Errorf("get guilds with job keys: %w", err)
	}
	for _, id := range withJobs {
		stored = append(stored, int64(id))
	}

	p := plan(listed, stored, departed, now)
	if len(p.gone) > 0 {
		if err := s.db.MarkDeparted(ctx, database.MarkDepartedParams{GuildIDs: p.gone, LeftAt: now}); err != nil {
			return fmt.Errorf("mark departed: %w", err)
		}
	}

	confirmed := s.confirmGone(ctx, p.due)
	if err := ctx.Err(); err != nil {
		return err
	}
	deleted, err := s.deleteGuilds(ctx, confirmed, now.Add(-retention))
	if err != nil {
		return err
	}
	if deleted > 0 || s.rewritePending {
		// The AOF keeps every earlier write of the deleted keys until Redis rewrites it.
		err := s.redis.BgRewriteAOF(ctx).Err()
		s.rewritePending = err != nil
		if err != nil {
			s.logger.Warn("rewrite redis AOF, retrying next sweep", zap.Error(err))
		}
	}

	s.logger.Info("sweep finished",
		zap.Int("listed", len(listed)),
		zap.Int("approximate_guild_count", approx),
		zap.Int("rejoined", len(back)),
		zap.Int("departed", len(p.gone)),
		zap.Int("due", len(p.due)),
		zap.Int("deleted", deleted),
	)
	return nil
}

// confirmGone asks Discord about each due guild, because a list that passed the 90% check can still miss one.
func (s *Sweeper) confirmGone(ctx context.Context, due []int64) []int64 {
	var gone []int64
	for _, id := range due {
		if ctx.Err() != nil {
			break
		}
		_, err := s.client.Rest.GetGuild(snowflake.ID(id), false, rest.WithCtx(ctx))
		switch {
		case err == nil:
			s.logger.Warn("due guild is still present, keeping it", zap.String("guild_id", strconv.FormatInt(id, 10)))
			if err := s.db.ClearDepartures(ctx, []int64{id}); err != nil {
				s.logger.Error("clear departure of present guild", zap.Error(err), zap.String("guild_id", strconv.FormatInt(id, 10)))
			}
		case rest.IsJSONErrorCode(err, rest.JSONErrorCodeMissingAccess, rest.JSONErrorCodeUnknownGuild):
			gone = append(gone, id)
		default:
			s.logger.Warn("confirm guild is gone, retrying next sweep", zap.Error(err), zap.String("guild_id", strconv.FormatInt(id, 10)))
		}
	}
	return gone
}

// Redis first: if Postgres then fails, the departure rows survive and the next sweep retries both.
func (s *Sweeper) deleteGuilds(ctx context.Context, guildIDs []int64, leftBefore time.Time) (int, error) {
	if len(guildIDs) == 0 {
		return 0, nil
	}
	ids := make([]uint64, len(guildIDs))
	for i, id := range guildIDs {
		ids[i] = uint64(id)
	}
	if err := job.DeleteGuilds(ctx, s.redis, ids); err != nil {
		return 0, fmt.Errorf("delete job keys: %w", err)
	}
	deleted, err := s.db.DeleteDepartedGuilds(ctx, database.DeleteDepartedGuildsParams{GuildIDs: guildIDs, LeftBefore: leftBefore})
	if err != nil {
		return 0, fmt.Errorf("delete guild data: %w", err)
	}
	for _, id := range deleted {
		s.logger.Info("deleted data of departed guild", zap.String("guild_id", strconv.FormatInt(id, 10)))
	}
	return len(deleted), nil
}
