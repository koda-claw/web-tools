package metrics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	apperrors "github.com/koda-claw/web-tools/internal/errors"
)

const (
	SchemaVersion        = 1
	DefaultRecentLimit   = 20
	DefaultHourRetention = 30 * 24 * time.Hour
	DefaultDayRetention  = 180 * 24 * time.Hour
)

// Event is the only accepted metrics input. It intentionally excludes query,
// URL, content, file path, headers, and detailed error strings.
type Event struct {
	At                  time.Time
	Command             string
	Provider            string
	Status              string
	ErrorCategory       string
	DurationMS          int64
	ResultCount         int
	WordCountBucket     string
	Quality             string
	FallbackRecommended bool
	Upgrade             UpgradeEvent
}

type UpgradeEvent struct {
	TargetVersion    string `json:"target_version,omitempty"`
	ChecksumVerified bool   `json:"checksum_verified,omitempty"`
	BinaryMode       string `json:"binary_mode,omitempty"`
}

type Snapshot struct {
	SchemaVersion int                `json:"schema_version"`
	GeneratedAt   time.Time          `json:"generated_at"`
	Period        Period             `json:"period"`
	Commands      map[string]Counter `json:"commands"`
	Providers     map[string]Counter `json:"providers"`
	ReaderQuality ReaderQuality      `json:"reader_quality"`
	Errors        map[string]int64   `json:"errors"`
	Upgrade       UpgradeSummary     `json:"upgrade,omitempty"`
	TimeBuckets   TimeBuckets        `json:"time_buckets"`
	RecentEvents  []RecentEvent      `json:"recent_events"`
	Disabled      bool               `json:"disabled,omitempty"`
	Warnings      []string           `json:"warnings,omitempty"`
}

type Period struct {
	FirstSeenAt time.Time `json:"first_seen_at,omitempty"`
	LastSeenAt  time.Time `json:"last_seen_at,omitempty"`
}

type Counter struct {
	Total           int64  `json:"total"`
	Success         int64  `json:"success"`
	Error           int64  `json:"error"`
	LastStatus      string `json:"last_status,omitempty"`
	LastDurationMS  int64  `json:"last_duration_ms,omitempty"`
	AvgDurationMS   int64  `json:"avg_duration_ms"`
	TotalDurationMS int64  `json:"total_duration_ms,omitempty"`
	ResultCount     int64  `json:"result_count,omitempty"`
}

type ReaderQuality struct {
	High                int64 `json:"high"`
	Medium              int64 `json:"medium"`
	Low                 int64 `json:"low"`
	FallbackRecommended int64 `json:"fallback_recommended"`
}

type UpgradeSummary struct {
	LastCheckAt          time.Time `json:"last_check_at,omitempty"`
	LastTargetVersion    string    `json:"last_target_version,omitempty"`
	LastChecksumVerified bool      `json:"last_checksum_verified,omitempty"`
	LastBinaryMode       string    `json:"last_binary_mode,omitempty"`
}

type TimeBuckets struct {
	Hour map[string]Bucket `json:"hour"`
	Day  map[string]Bucket `json:"day"`
}

type Bucket struct {
	Commands      map[string]Counter `json:"commands,omitempty"`
	Providers     map[string]Counter `json:"providers,omitempty"`
	ReaderQuality ReaderQuality      `json:"reader_quality,omitempty"`
	Errors        map[string]int64   `json:"errors,omitempty"`
}

type RecentEvent struct {
	At                  time.Time `json:"at"`
	Command             string    `json:"command"`
	Status              string    `json:"status"`
	DurationMS          int64     `json:"duration_ms"`
	Provider            string    `json:"provider,omitempty"`
	Quality             string    `json:"quality,omitempty"`
	ErrorCategory       string    `json:"error_category,omitempty"`
	FallbackRecommended bool      `json:"fallback_recommended,omitempty"`
}

type Range string

const (
	Range1H  Range = "1h"
	Range24H Range = "24h"
	Range7D  Range = "7d"
	Range30D Range = "30d"
	RangeAll Range = "all"
)

type BucketKind string

const (
	BucketAuto BucketKind = "auto"
	BucketHour BucketKind = "hour"
	BucketDay  BucketKind = "day"
)

type Store struct {
	Path string
	Now  func() time.Time
}

func DefaultPath() string {
	if p := strings.TrimSpace(os.Getenv("WEB_TOOLS_METRICS_FILE")); p != "" {
		return expandHome(p)
	}
	switch runtime.GOOS {
	case "windows":
		if base := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); base != "" {
			return filepath.Join(base, "web-tools", "metrics.json")
		}
		return filepath.Join(expandHome("~"), "AppData", "Local", "web-tools", "metrics.json")
	case "darwin":
		return filepath.Join(expandHome("~"), "Library", "Application Support", "web-tools", "metrics.json")
	default:
		if base := strings.TrimSpace(os.Getenv("XDG_STATE_HOME")); base != "" {
			return filepath.Join(base, "web-tools", "metrics.json")
		}
		return filepath.Join(expandHome("~"), ".local", "state", "web-tools", "metrics.json")
	}
}

func Disabled() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("WEB_TOOLS_NO_METRICS")))
	return v == "1" || v == "true" || v == "yes"
}

func NewStore(path string) Store {
	if path == "" {
		path = DefaultPath()
	}
	return Store{Path: expandHome(path), Now: time.Now}
}

func (s Store) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s Store) Snapshot(r Range, b BucketKind) (Snapshot, error) {
	snap, err := s.load()
	if err != nil {
		return Snapshot{}, err
	}
	snap.GeneratedAt = s.now()
	return Filter(snap, r, b, s.now()), nil
}

func (s Store) Record(event Event) error {
	if Disabled() {
		return nil
	}
	snap, err := s.load()
	if err != nil {
		return err
	}
	now := s.now()
	if event.At.IsZero() {
		event.At = now
	}
	applyEvent(&snap, sanitizeEvent(event))
	pruneBuckets(&snap, now)
	return s.save(snap)
}

func (s Store) Reset() error {
	if Disabled() {
		return nil
	}
	return s.save(newSnapshot())
}

func ObserveCommand(start time.Time, command string, attrs Event, err error) {
	if Disabled() || command == "" {
		return
	}
	event := attrs
	event.Command = command
	if event.At.IsZero() {
		event.At = time.Now()
	}
	event.DurationMS = time.Since(start).Milliseconds()
	if err != nil {
		event.Status = "error"
		if event.ErrorCategory == "" {
			event.ErrorCategory = ErrorCategory(err)
		}
	} else {
		event.Status = "success"
	}
	_ = NewStore("").Record(event)
}

func ErrorCategory(err error) string {
	if err == nil {
		return ""
	}
	var appErr *apperrors.AppError
	if apperrors.As(err, &appErr) && appErr.Category != "" {
		return safeID(appErr.Category)
	}
	msg := err.Error()
	if strings.Contains(msg, "[network]") || strings.Contains(msg, "network") {
		return "network"
	}
	if strings.Contains(msg, "[extract]") || strings.Contains(msg, "extract") {
		return "extract"
	}
	if strings.Contains(msg, "[engine]") || strings.Contains(msg, "engine") {
		return "engine"
	}
	if strings.Contains(msg, "[input]") || strings.Contains(msg, "input") {
		return "input"
	}
	return "internal"
}

func Filter(snap Snapshot, r Range, b BucketKind, now time.Time) Snapshot {
	if r == "" {
		r = RangeAll
	}
	if b == "" || b == BucketAuto {
		if r == Range1H || r == Range24H {
			b = BucketHour
		} else {
			b = BucketDay
		}
	}
	if r == RangeAll {
		snap.GeneratedAt = now
		return snap
	}
	cutoff, ok := rangeCutoff(r, now)
	if !ok {
		snap.Warnings = append(snap.Warnings, "invalid range")
		return snap
	}
	filtered := newSnapshot()
	filtered.GeneratedAt = now
	filtered.TimeBuckets = TimeBuckets{Hour: map[string]Bucket{}, Day: map[string]Bucket{}}
	source := snap.TimeBuckets.Day
	if b == BucketHour {
		source = snap.TimeBuckets.Hour
	}
	keys := sortedBucketKeys(source)
	for _, key := range keys {
		t, err := parseBucketTime(key, b)
		if err != nil || t.Before(cutoff) {
			continue
		}
		mergeBucket(&filtered, source[key])
		updatePeriod(&filtered.Period, t)
		if b == BucketHour {
			filtered.TimeBuckets.Hour[key] = source[key]
		} else {
			filtered.TimeBuckets.Day[key] = source[key]
		}
	}
	for _, event := range snap.RecentEvents {
		if !event.At.Before(cutoff) {
			filtered.RecentEvents = append(filtered.RecentEvents, event)
			updatePeriod(&filtered.Period, event.At)
		}
	}
	return filtered
}

func ParseRange(raw string) (Range, error) {
	if raw == "" {
		return RangeAll, nil
	}
	switch Range(raw) {
	case Range1H, Range24H, Range7D, Range30D, RangeAll:
		return Range(raw), nil
	default:
		return "", fmt.Errorf("unsupported range %q", raw)
	}
}

func ParseBucket(raw string) (BucketKind, error) {
	if raw == "" {
		return BucketAuto, nil
	}
	switch BucketKind(raw) {
	case BucketAuto, BucketHour, BucketDay:
		return BucketKind(raw), nil
	default:
		return "", fmt.Errorf("unsupported bucket %q", raw)
	}
}

func newSnapshot() Snapshot {
	return Snapshot{
		SchemaVersion: SchemaVersion,
		GeneratedAt:   time.Now(),
		Commands:      map[string]Counter{},
		Providers:     map[string]Counter{},
		Errors:        map[string]int64{},
		TimeBuckets: TimeBuckets{
			Hour: map[string]Bucket{},
			Day:  map[string]Bucket{},
		},
		RecentEvents: []RecentEvent{},
	}
}

func (s Store) load() (Snapshot, error) {
	if Disabled() {
		snap := newSnapshot()
		snap.Disabled = true
		return snap, nil
	}
	data, err := os.ReadFile(s.Path)
	if os.IsNotExist(err) {
		return newSnapshot(), nil
	}
	if err != nil {
		return Snapshot{}, err
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		backup := fmt.Sprintf("%s.corrupt.%d", s.Path, s.now().Unix())
		_ = os.Rename(s.Path, backup)
		fresh := newSnapshot()
		fresh.Warnings = append(fresh.Warnings, "metrics file was corrupt and has been reset")
		return fresh, nil
	}
	ensureSnapshot(&snap)
	return snap, nil
}

func (s Store) save(snap Snapshot) error {
	ensureSnapshot(&snap)
	if err := os.MkdirAll(filepath.Dir(s.Path), 0755); err != nil {
		return err
	}
	snap.GeneratedAt = s.now()
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.Path), ".metrics-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, s.Path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func ensureSnapshot(s *Snapshot) {
	if s.SchemaVersion == 0 {
		s.SchemaVersion = SchemaVersion
	}
	if s.Commands == nil {
		s.Commands = map[string]Counter{}
	}
	if s.Providers == nil {
		s.Providers = map[string]Counter{}
	}
	if s.Errors == nil {
		s.Errors = map[string]int64{}
	}
	if s.TimeBuckets.Hour == nil {
		s.TimeBuckets.Hour = map[string]Bucket{}
	}
	if s.TimeBuckets.Day == nil {
		s.TimeBuckets.Day = map[string]Bucket{}
	}
	if s.RecentEvents == nil {
		s.RecentEvents = []RecentEvent{}
	}
}

func sanitizeEvent(e Event) Event {
	e.Command = safeID(e.Command)
	e.Provider = safeID(e.Provider)
	e.Status = safeStatus(e.Status)
	e.ErrorCategory = safeID(e.ErrorCategory)
	e.Quality = safeQuality(e.Quality)
	e.WordCountBucket = safeID(e.WordCountBucket)
	if e.DurationMS < 0 {
		e.DurationMS = 0
	}
	if e.ResultCount < 0 {
		e.ResultCount = 0
	}
	return e
}

func applyEvent(s *Snapshot, e Event) {
	ensureSnapshot(s)
	updatePeriod(&s.Period, e.At)
	updateCounter(s.Commands, e.Command, e)
	if e.Provider != "" {
		updateCounter(s.Providers, providerKey(e.Command, e.Provider), e)
	}
	if e.ErrorCategory != "" && e.Status == "error" {
		s.Errors[e.ErrorCategory]++
	}
	applyQuality(&s.ReaderQuality, e)
	if e.Command == "upgrade" {
		s.Upgrade.LastCheckAt = e.At
		s.Upgrade.LastTargetVersion = e.Upgrade.TargetVersion
		s.Upgrade.LastChecksumVerified = e.Upgrade.ChecksumVerified
		s.Upgrade.LastBinaryMode = e.Upgrade.BinaryMode
	}
	addToBucket(s.TimeBuckets.Hour, hourKey(e.At), e)
	addToBucket(s.TimeBuckets.Day, dayKey(e.At), e)
	s.RecentEvents = append(s.RecentEvents, RecentEvent{
		At:                  e.At,
		Command:             e.Command,
		Status:              e.Status,
		DurationMS:          e.DurationMS,
		Provider:            e.Provider,
		Quality:             e.Quality,
		ErrorCategory:       e.ErrorCategory,
		FallbackRecommended: e.FallbackRecommended,
	})
	if len(s.RecentEvents) > DefaultRecentLimit {
		s.RecentEvents = s.RecentEvents[len(s.RecentEvents)-DefaultRecentLimit:]
	}
}

func updatePeriod(p *Period, t time.Time) {
	if t.IsZero() {
		return
	}
	if p.FirstSeenAt.IsZero() || t.Before(p.FirstSeenAt) {
		p.FirstSeenAt = t
	}
	if p.LastSeenAt.IsZero() || t.After(p.LastSeenAt) {
		p.LastSeenAt = t
	}
}

func updateCounter(m map[string]Counter, key string, e Event) {
	if key == "" {
		return
	}
	c := m[key]
	c.Total++
	if e.Status == "error" {
		c.Error++
	} else {
		c.Success++
	}
	c.LastStatus = e.Status
	c.LastDurationMS = e.DurationMS
	c.TotalDurationMS += e.DurationMS
	if e.ResultCount > 0 {
		c.ResultCount += int64(e.ResultCount)
	}
	if c.Total > 0 {
		c.AvgDurationMS = c.TotalDurationMS / c.Total
	}
	m[key] = c
}

func applyQuality(q *ReaderQuality, e Event) {
	switch e.Quality {
	case "high":
		q.High++
	case "medium":
		q.Medium++
	case "low":
		q.Low++
	}
	if e.FallbackRecommended {
		q.FallbackRecommended++
	}
}

func addToBucket(buckets map[string]Bucket, key string, e Event) {
	b := buckets[key]
	if b.Commands == nil {
		b.Commands = map[string]Counter{}
	}
	if b.Providers == nil {
		b.Providers = map[string]Counter{}
	}
	if b.Errors == nil {
		b.Errors = map[string]int64{}
	}
	updateCounter(b.Commands, e.Command, e)
	if e.Provider != "" {
		updateCounter(b.Providers, providerKey(e.Command, e.Provider), e)
	}
	if e.ErrorCategory != "" && e.Status == "error" {
		b.Errors[e.ErrorCategory]++
	}
	applyQuality(&b.ReaderQuality, e)
	buckets[key] = b
}

func mergeBucket(s *Snapshot, b Bucket) {
	for key, counter := range b.Commands {
		mergeCounter(s.Commands, key, counter)
	}
	for key, counter := range b.Providers {
		mergeCounter(s.Providers, key, counter)
	}
	for key, count := range b.Errors {
		s.Errors[key] += count
	}
	s.ReaderQuality.High += b.ReaderQuality.High
	s.ReaderQuality.Medium += b.ReaderQuality.Medium
	s.ReaderQuality.Low += b.ReaderQuality.Low
	s.ReaderQuality.FallbackRecommended += b.ReaderQuality.FallbackRecommended
}

func mergeCounter(m map[string]Counter, key string, c Counter) {
	existing := m[key]
	existing.Total += c.Total
	existing.Success += c.Success
	existing.Error += c.Error
	existing.TotalDurationMS += c.TotalDurationMS
	existing.ResultCount += c.ResultCount
	existing.LastStatus = c.LastStatus
	existing.LastDurationMS = c.LastDurationMS
	if existing.Total > 0 {
		existing.AvgDurationMS = existing.TotalDurationMS / existing.Total
	}
	m[key] = existing
}

func pruneBuckets(s *Snapshot, now time.Time) {
	pruneBucketMap(s.TimeBuckets.Hour, now.Add(-DefaultHourRetention), BucketHour)
	pruneBucketMap(s.TimeBuckets.Day, now.Add(-DefaultDayRetention), BucketDay)
}

func pruneBucketMap(m map[string]Bucket, cutoff time.Time, kind BucketKind) {
	for key := range m {
		t, err := parseBucketTime(key, kind)
		if err == nil && t.Before(cutoff) {
			delete(m, key)
		}
	}
}

func rangeCutoff(r Range, now time.Time) (time.Time, bool) {
	switch r {
	case Range1H:
		return now.Add(-time.Hour), true
	case Range24H:
		return now.Add(-24 * time.Hour), true
	case Range7D:
		return now.Add(-7 * 24 * time.Hour), true
	case Range30D:
		return now.Add(-30 * 24 * time.Hour), true
	default:
		return time.Time{}, false
	}
}

func sortedBucketKeys(m map[string]Bucket) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func parseBucketTime(key string, kind BucketKind) (time.Time, error) {
	if kind == BucketHour {
		return time.Parse(time.RFC3339, key)
	}
	return time.Parse("2006-01-02", key)
}

func hourKey(t time.Time) string {
	return t.UTC().Truncate(time.Hour).Format(time.RFC3339)
}

func dayKey(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

func providerKey(command, provider string) string {
	if strings.Contains(command, "search") {
		return "search:" + provider
	}
	if strings.Contains(command, "reader") {
		return "reader:" + provider
	}
	return command + ":" + provider
}

func safeID(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	if v == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range v {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == ':' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func safeStatus(v string) string {
	if v == "error" {
		return "error"
	}
	return "success"
}

func safeQuality(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "high", "medium", "low":
		return strings.ToLower(strings.TrimSpace(v))
	default:
		return ""
	}
}

func expandHome(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}
