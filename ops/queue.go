package ops

import (
	"fmt"
	"strings"
	"time"
)

// QueueStep represents a single step in an operation queue.
type QueueStep struct {
	Name   string // operation name (distribute, collect, swap, bridge, etc.)
	Params map[string]string
}

// QueueConfig holds a sequence of operations to execute.
type QueueConfig struct {
	Steps       []QueueStep
	DelayBetween DelayConfig // delay between steps
}

// QueueResult holds the result of a single queue step execution.
type QueueResult struct {
	StepIndex int
	StepName  string
	Success   bool
	Message   string
	Duration  time.Duration
}

// NewQueue creates an empty queue.
func NewQueue() *QueueConfig {
	return &QueueConfig{
		Steps: make([]QueueStep, 0),
	}
}

// AddStep appends a step to the queue.
func (q *QueueConfig) AddStep(name string, params map[string]string) {
	q.Steps = append(q.Steps, QueueStep{Name: name, Params: params})
}

// Describe returns a human-readable description of the queue.
func (q *QueueConfig) Describe() string {
	if len(q.Steps) == 0 {
		return "Empty queue"
	}
	var parts []string
	for i, s := range q.Steps {
		parts = append(parts, fmt.Sprintf("%d. %s", i+1, s.Name))
	}
	return strings.Join(parts, " → ")
}

// Preset represents a saved operation configuration.
type Preset struct {
	Name        string            `json:"name"`
	Operation   string            `json:"operation"`
	Params      map[string]string `json:"params"`
	CreatedAt   time.Time         `json:"created_at"`
}

// PresetStore manages saved presets.
type PresetStore struct {
	Presets []Preset `json:"presets"`
}

// NewPresetStore creates an empty preset store.
func NewPresetStore() *PresetStore {
	return &PresetStore{Presets: make([]Preset, 0)}
}

// Add saves a new preset.
func (ps *PresetStore) Add(name, operation string, params map[string]string) {
	ps.Presets = append(ps.Presets, Preset{
		Name:      name,
		Operation: operation,
		Params:    params,
		CreatedAt: time.Now(),
	})
}

// Remove deletes a preset by name.
func (ps *PresetStore) Remove(name string) bool {
	for i, p := range ps.Presets {
		if p.Name == name {
			ps.Presets = append(ps.Presets[:i], ps.Presets[i+1:]...)
			return true
		}
	}
	return false
}

// Get returns a preset by name.
func (ps *PresetStore) Get(name string) *Preset {
	for i := range ps.Presets {
		if ps.Presets[i].Name == name {
			return &ps.Presets[i]
		}
	}
	return nil
}

// Names returns a list of all preset names.
func (ps *PresetStore) Names() []string {
	names := make([]string, len(ps.Presets))
	for i, p := range ps.Presets {
		names[i] = p.Name
	}
	return names
}
