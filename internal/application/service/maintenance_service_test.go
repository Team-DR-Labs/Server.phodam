package service

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/dating"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/photo"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/push"
)

func TestMaintenance_ExpireAfter72hCleansUnsubmittedPhotos(t *testing.T) {
	h := newHarness(t)
	a, b := h.newCouple()
	dateID := h.startJoined(t, a, b)
	rep, aExtra := h.shoot(t, a, dateID), h.shoot(t, a, dateID)
	h.submit(t, a, dateID, rep, "")
	bUploaded := h.shoot(t, b, dateID)
	bReserved, err := NewPhotoService(h.deps).ReserveShot(ctx, b, dateID)
	mustOK(t, err)
	m := NewMaintenanceService(h.deps)

	h.clock.Advance(dating.SubmitWindow - time.Second)
	m.RunOnce(ctx)
	if h.st.dates[dateID].Status != dating.StatusInProgress {
		t.Fatal("must not expire before 72h")
	}

	h.clock.Advance(time.Second)
	m.RunOnce(ctx)

	if h.st.dates[dateID].Status != dating.StatusExpired {
		t.Fatalf("status = %s, want expired", h.st.dates[dateID].Status)
	}
	for _, id := range []photoIDStatus{{bUploaded, photo.StatusDeleted}, {bReserved.Photo.ID, photo.StatusDeleted}, {rep, photo.StatusArchived}, {aExtra, photo.StatusUploaded}} {
		if got := h.st.photos[id.id].Status; got != id.want {
			t.Fatalf("photo %s status = %s, want %s", id.id, got, id.want)
		}
	}
	if h.storage.has(out.BucketTemp, h.st.photos[bUploaded].TempKey) {
		t.Fatal("unsubmitted participant's object must be deleted")
	}
	got, err := NewPhotoService(h.deps).Receivable(ctx, a, dateID)
	mustOK(t, err)
	if len(got.Items) != 2 {
		t.Fatal("submitted participant keeps receiving after expiry")
	}
}

func TestMaintenance_ReceiveDeadlineDeletesUploadedButKeepsArchived(t *testing.T) {
	h := newHarness(t)
	a, b := h.newCouple()
	dateID := h.startJoined(t, a, b)
	rep, extra := h.shoot(t, a, dateID), h.shoot(t, a, dateID)
	h.submit(t, a, dateID, rep, "")
	m := NewMaintenanceService(h.deps)

	h.clock.Advance(dating.ReceiveWindow - time.Second)
	m.RunOnce(ctx)
	if h.st.photos[extra].Status != photo.StatusUploaded {
		t.Fatal("must keep photo before receive deadline")
	}
	h.clock.Advance(time.Second)
	m.RunOnce(ctx)

	if h.st.photos[extra].Status != photo.StatusDeleted || h.storage.has(out.BucketTemp, h.st.photos[extra].TempKey) {
		t.Fatalf("overdue uploaded photo must be deleted: %+v", h.st.photos[extra])
	}
	if h.st.photos[rep].Status != photo.StatusArchived || !h.storage.has(out.BucketPermanent, photo.PermanentKey(dateID, a, rep)) {
		t.Fatal("archived representative must be kept")
	}
}

func TestMaintenance_StorageFailureRetriesNextCycle(t *testing.T) {
	h := newHarness(t)
	a, b := h.newCouple()
	dateID := h.startJoined(t, a, b)
	bUploaded := h.shoot(t, b, dateID)
	m := NewMaintenanceService(h.deps)

	h.clock.Advance(dating.SubmitWindow)
	h.storage.failRemove = true
	m.RunOnce(ctx)
	if h.st.photos[bUploaded].Status != photo.StatusUploaded {
		t.Fatal("status must not change when object delete fails")
	}
	h.storage.failRemove = false
	m.RunOnce(ctx)
	if h.st.photos[bUploaded].Status != photo.StatusDeleted {
		t.Fatal("next cycle retries the delete")
	}
}

func TestMaintenance_DeadlineReminderOnceToUnsubmitted(t *testing.T) {
	h := newHarness(t)
	a, b := h.newCouple()
	dateID := h.startJoined(t, a, b)
	h.submit(t, a, dateID, h.shoot(t, a, dateID), "")
	m := NewMaintenanceService(h.deps)

	h.clock.Advance(dating.SubmitWindow - dating.ReminderLead - time.Second)
	m.RunOnce(ctx)
	if h.st.dates[dateID].RemindedAt != nil {
		t.Fatal("too early for reminder")
	}
	h.clock.Advance(time.Second)
	m.RunOnce(ctx)
	m.RunOnce(ctx)

	if count(h.push.typesTo("tok-bob"), push.TypeDeadlineSoon) != 1 {
		t.Fatalf("bob gets exactly one reminder: %v", h.push.typesTo("tok-bob"))
	}
	if count(h.push.typesTo("tok-alice"), push.TypeDeadlineSoon) != 0 {
		t.Fatal("submitted participant gets no reminder")
	}
}

func TestMaintenance_RetriesArchivedTempPurge(t *testing.T) {
	h := newHarness(t)
	a, b := h.newCouple()
	dateID := h.startJoined(t, a, b)
	rep := h.shoot(t, a, dateID)
	h.storage.failRemove = true
	h.submit(t, a, dateID, rep, "")
	h.storage.failRemove = false

	NewMaintenanceService(h.deps).RunOnce(ctx)

	p := h.st.photos[rep]
	if p.TempPurgedAt == nil || h.storage.has(out.BucketTemp, p.TempKey) || p.Status != photo.StatusArchived {
		t.Fatalf("temp copy of archived photo must be purged: %+v", p)
	}
}

type photoIDStatus struct {
	id   uuid.UUID
	want photo.Status
}

func count(types []push.Type, t push.Type) int {
	n := 0
	for _, x := range types {
		if x == t {
			n++
		}
	}
	return n
}
