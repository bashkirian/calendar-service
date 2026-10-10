package sqs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecode_RejectsUnknownVersion(t *testing.T) {
	_, err := decode(`{"type":"calendar.entry.active","version":"2","data":{}}`)
	require.ErrorContains(t, err, "unsupported message")
}
