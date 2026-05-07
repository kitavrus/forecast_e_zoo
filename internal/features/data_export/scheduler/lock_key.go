// Package scheduler — gocron-обёртка вокруг loader-а с PG advisory lock,
// pre-step созданием месячных партиций и stale-load reaper-ом.
package scheduler

import "hash/fnv"

// LockTagDailyLoad — стабильный тег для daily-load advisory lock.
const LockTagDailyLoad = "source-adapter:daily-load"

// LockKey возвращает детерминированный bigint ключ для pg_try_advisory_lock.
func LockKey(tag string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(tag))
	return int64(h.Sum64()) //nolint:gosec // ok: postgres accepts full int64 range
}
