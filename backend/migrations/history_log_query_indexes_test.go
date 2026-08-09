package migrations

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHistoryLogQueryIndexesMigration(t *testing.T) {
	sql, err := os.ReadFile("sql/20260809_120000_history_log_query_indexes.sql")
	require.NoError(t, err)

	contents := string(sql)
	require.Contains(t, contents, "termlogs(flow_id, created_at, id)")
	require.Contains(t, contents, "assistantlogs(flow_id, assistant_id, created_at, id)")
	require.Equal(t, 2, strings.Count(contents, "CREATE INDEX IF NOT EXISTS"))
}
