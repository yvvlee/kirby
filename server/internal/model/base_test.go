package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPrepareInsertPreservesExplicitInstantsAndNormalizesUTC(t *testing.T) {
	zone := time.FixedZone("integration UTC+8", 8*60*60)
	createdAt := time.Date(2024, time.June, 3, 17, 21, 19, 123000, zone)
	updatedAt := createdAt.Add(time.Hour)
	meta := Meta{
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		Version:   99,
	}
	meta.PrepareInsert()
	require.True(t, createdAt.Equal(meta.CreatedAt))
	require.True(t, updatedAt.Equal(meta.UpdatedAt))
	require.Same(t, time.UTC, meta.CreatedAt.Location())
	require.Same(t, time.UTC, meta.UpdatedAt.Location())
	require.EqualValues(t, 1, meta.Version)
	record := RecordMeta{
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}
	record.PrepareInsert()
	require.Equal(t, meta.CreatedAt, record.CreatedAt)
	require.Equal(t, meta.UpdatedAt, record.UpdatedAt)
	workflow := WorkflowMeta{
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}
	workflow.PrepareInsert()
	require.Equal(t, meta.CreatedAt, workflow.CreatedAt)
	require.Equal(t, meta.UpdatedAt, workflow.UpdatedAt)
	require.EqualValues(t, 1, workflow.Version)
}

func TestPrepareInsertFillsMissingTimesWithoutReplacingSuppliedTime(t *testing.T) {
	explicit := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)
	meta := Meta{
		CreatedAt: explicit,
	}
	meta.PrepareInsert()
	require.Equal(t, explicit, meta.CreatedAt)
	require.WithinDuration(t, time.Now(), meta.UpdatedAt, time.Second)
	record := RecordMeta{
		UpdatedAt: explicit,
	}
	record.PrepareInsert()
	require.Equal(t, explicit, record.UpdatedAt)
	require.WithinDuration(t, time.Now(), record.CreatedAt, time.Second)
	workflow := WorkflowMeta{}
	workflow.PrepareInsert()
	require.Equal(t, workflow.CreatedAt, workflow.UpdatedAt)
	require.WithinDuration(t, time.Now(), workflow.CreatedAt, time.Second)
	require.Same(t, time.UTC, workflow.CreatedAt.Location())
}
