package task

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStoreListWithFilters(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	base := time.Now().Add(-2 * time.Minute)

	tasks := []*Task{
		{ID: "t1", Goal: "g1", Status: StatusPending, MaxRetries: 3},
		{ID: "t2", Goal: "g2", Status: StatusFailed, MaxRetries: 3},
		{ID: "t3", Goal: "g3", Status: StatusSucceeded, MaxRetries: 3},
	}

	for _, task := range tasks {
		if err := store.Create(ctx, task); err != nil {
			t.Fatalf("create task %s: %v", task.ID, err)
		}
		time.Sleep(5 * time.Millisecond)
	}

	if err := store.MarkFailed(ctx, "t2", CodeTaskProcessing, "boom", true); err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	if err := store.MarkSucceeded(ctx, "t3", ExecutionResult{Reply: "ok"}); err != nil {
		t.Fatalf("mark succeeded: %v", err)
	}

	store.mu.Lock()
	store.tasks["t1"].UpdatedAt = base.Unix()
	store.tasks["t2"].UpdatedAt = base.Add(30 * time.Second).Unix()
	store.tasks["t3"].UpdatedAt = base.Add(60 * time.Second).Unix()
	store.mu.Unlock()

	all, err := store.List(ctx, ListOptions{})
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(all))
	}
	if all[0].ID != "t3" {
		t.Fatalf("expected newest task first, got %s", all[0].ID)
	}

	failed, err := store.List(ctx, buildListOptions([]ListOption{WithStatuses(StatusFailed)}))
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(failed) != 1 || failed[0].ID != "t2" {
		t.Fatalf("unexpected failed list: %+v", failed)
	}

	succeeded, err := store.List(ctx, buildListOptions([]ListOption{WithResultPresence(true)}))
	if err != nil {
		t.Fatalf("list with result: %v", err)
	}
	if len(succeeded) != 1 || succeeded[0].ID != "t3" {
		t.Fatalf("unexpected result list: %+v", succeeded)
	}

	since := base.Add(15 * time.Second)
	recent, err := store.List(ctx, buildListOptions([]ListOption{WithUpdatedSince(since)}))
	if err != nil {
		t.Fatalf("list recent: %v", err)
	}
	if len(recent) != 2 {
		t.Fatalf("expected 2 tasks to match since filter, got %d", len(recent))
	}
}

func TestMemoryStoreStats(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	base := time.Now().Add(-3 * time.Minute)
	tasks := []*Task{
		{ID: "a", Goal: "g1", Status: StatusPending, MaxRetries: 3},
		{ID: "b", Goal: "g2", Status: StatusPending, MaxRetries: 3},
		{ID: "c", Goal: "g3", Status: StatusPending, MaxRetries: 3},
	}

	for _, task := range tasks {
		if err := store.Create(ctx, task); err != nil {
			t.Fatalf("create task %s: %v", task.ID, err)
		}
		time.Sleep(2 * time.Millisecond)
	}

	if err := store.MarkFailed(ctx, "b", CodeTaskProcessing, "boom", true); err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	if err := store.MarkSucceeded(ctx, "c", ExecutionResult{Reply: "ok"}); err != nil {
		t.Fatalf("mark succeeded: %v", err)
	}

	store.mu.Lock()
	store.tasks["a"].UpdatedAt = base.Unix()
	store.tasks["b"].UpdatedAt = base.Add(30 * time.Second).Unix()
	store.tasks["c"].UpdatedAt = base.Add(2 * time.Minute).Unix()
	store.mu.Unlock()

	stats, err := store.Stats(ctx, ListOptions{})
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.Total != 3 || stats.Pending != 1 || stats.Failed != 1 || stats.Succeeded != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if stats.NewestUpdatedAt != base.Add(2*time.Minute).Unix() {
		t.Fatalf("unexpected newest timestamp: %d", stats.NewestUpdatedAt)
	}
	if stats.OldestUpdatedAt != base.Unix() {
		t.Fatalf("unexpected oldest timestamp: %d", stats.OldestUpdatedAt)
	}

	withResults, err := store.Stats(ctx, buildListOptions([]ListOption{WithResultPresence(true)}))
	if err != nil {
		t.Fatalf("stats with result: %v", err)
	}
	if withResults.Total != 1 || withResults.Succeeded != 1 {
		t.Fatalf("unexpected stats with result: %+v", withResults)
	}

	withoutResults, err := store.Stats(ctx, buildListOptions([]ListOption{WithResultPresence(false)}))
	if err != nil {
		t.Fatalf("stats without result: %v", err)
	}
	if withoutResults.Total != 2 || withoutResults.Pending != 1 || withoutResults.Failed != 1 {
		t.Fatalf("unexpected stats without result: %+v", withoutResults)
	}

	failedOnly, err := store.Stats(ctx, buildListOptions([]ListOption{WithStatuses(StatusFailed)}))
	if err != nil {
		t.Fatalf("stats failed only: %v", err)
	}
	if failedOnly.Total != 1 || failedOnly.Failed != 1 {
		t.Fatalf("unexpected failed stats: %+v", failedOnly)
	}
}
