package utils

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/disgoorg/disgo/sharding"
	"github.com/disgoorg/json/v2"
)

const shardStateFileName = "shards.json"

func LoadShardStates(flagDir string) (map[int]sharding.ShardState, bool) {
	path := filepath.Join(flagDir, shardStateFileName)

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}

	var states map[int]sharding.ShardState
	if err := json.Unmarshal(raw, &states); err != nil {
		return nil, false
	}

	if len(states) == 0 {
		return nil, false
	}

	return states, true
}

func SaveShardStates(flagDir string, states map[int]sharding.ShardState) error {
	if len(states) == 0 {
		return nil
	}

	if err := os.MkdirAll(flagDir, 0o750); err != nil {
		return err
	}

	path := filepath.Join(flagDir, shardStateFileName)

	raw, err := json.Marshal(states)
	if err != nil {
		return err
	}

	return os.WriteFile(path, raw, 0o600)
}

func ShardStatesFromManager(manager sharding.ShardManager) (map[int]sharding.ShardState, error) {
	if manager == nil {
		return nil, errors.New("shard manager is nil")
	}

	out := map[int]sharding.ShardState{}

	for shard := range manager.Shards() {
		if shard == nil {
			continue
		}

		sessionID := shard.SessionID()
		seq := shard.LastSequenceReceived()

		resumeURL := shard.ResumeURL()
		if sessionID == nil || seq == nil || resumeURL == nil {
			continue
		}

		out[shard.ShardID()] = sharding.ShardState{
			SessionID: *sessionID,
			Sequence:  *seq,
			ResumeURL: *resumeURL,
		}
	}

	if len(out) == 0 {
		return nil, nil
	}

	return out, nil
}
