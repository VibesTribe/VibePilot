package core

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "sync"
    "time"
)

"github.com/vibepilot/governor/internal/db"
)

type Event struct {
    ID        string
    Type      EventType
    EntityID  string
    Timestamp time.Time
    Details   map[string]any
}

type EventCallback func(event Event)

type StateMachine struct {
    db       *db.DB
    mu      sync.RWMutex
    state   *SystemState
    events  []Event
    callbacks []EventCallback
}

func NewStateMachine(database *db.DB) *StateMachine {
    sm := &StateMachine{
        db:     database,
        state:  &SystemState{
            Version:   "1.0",
            UpdatedAt: time.Now(),
            Agents:    []AgentState{},
            Plans:    []PlanState{},
            Tasks:    []TaskState{},
            Slices:   []SliceState{},
            Failures: []FailureState{},
            Learning: LearningState{
                ModelScores:       make(map[string]ModelScore),
                PatternDetection: []Pattern{},
                Improvements:     []ImprovementSuggestion{},
                LastAnalysis:      time.Now(),
            },
        },
        events: []Event{},
    }
    
    if err := sm.load(); err != nil {
        log.Printf("[StateMachine] Failed to load initial state: %v", err)
    }
    
    return sm
}

func (sm *StateMachine) load() error {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    
    // Load from database
    stateJSON, err := sm.db.Query(context.Background(), "system_state", map[string]any{
        "key": "current",
    })
    if err != nil {
        return err
    }
    
    if len(stateJSON) > 0 {
        if err := json.Unmarshal(stateJSON, sm.state); err != nil {
            return err
        }
    }
    
    return nil
}
func (sm *StateMachine) save() error {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    
    stateJSON, err := json.Marshal(sm.state)
    if err != nil {
        return err
    }
    
    // Persist to database
    _, err = sm.db.RPC(context.Background(), "save_system_state", map[string]any{
        "p_state": json.RawMessage(stateJSON),
    })
    if err != nil {
        return err
    }
    
    return nil
}
func (sm *StateMachine) Apply(event Event) error {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    
    // 1. Append to event log
    sm.events = append(sm.events, event)
    
    // 2. Apply event to state
    if err := sm.applyEvent(event); err != nil {
        log.Printf("[StateMachine] Failed to apply event %s: %v", event.Type, err)
        return err
    }
    
    // 3. Persist
    if err := sm.save(); err != nil {
        log.Printf("[StateMachine] Failed to save state: %v", err)
        return err
    }
    
    // 4. Notify callbacks
    for _, callback := range sm.callbacks {
        callback(event)
    }
    
    return nil
}
func (sm *StateMachine) applyEvent(event Event) error {
    switch event.Type {
    case EventPlanCreated:
        return sm.handlePlanCreated(event)
    case EventTaskClaimed:
        return sm.handleTaskClaimed(event)
    case EventTaskCheckpoint:
        return sm.handleTaskCheckpoint(event)
    case EventTaskCompleted:
        return sm.handleTaskCompleted(event)
    case EventTaskError:
        return sm.handleTaskError(event)
    case EventModelSuccess:
        return sm.handleModelSuccess(event)
    case EventModelFailure:
        return sm.handleModelFailure(event)
    default:
        log.Printf("[StateMachine] Unknown event type: %s", event.Type)
    }
    return nil
}
func (sm *StateMachine) handleTaskCheckpoint(event Event) error {
    taskID, ok := event.Details["task_id"].(string)
    progress, ok := event.Details["progress"].(int)
    output, _ := event.Details["output"].(string)
    
    for i, range sm.state.Tasks {
        if sm.state.Tasks[i].ID == taskID {
            if sm.state.Tasks[i].Checkpoint == nil {
                sm.state.Tasks[i].Checkpoint = &Checkpoint{}
            }
            sm.state.Tasks[i].Checkpoint.Progress = progress
            sm.state.Tasks[i].Checkpoint.Output = output
            sm.state.Tasks[i].Checkpoint.Timestamp = time.Now()
            sm.state.Tasks[i].Progress = progress
            sm.state.UpdatedAt = time.Now()
            break
        }
    }
    
    return nil
}
func (sm *StateMachine) Recover() []TaskState {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    
    var recoverable []TaskState
    
    // Find all in_progress tasks with checkpoints
    for _, task := range sm.state.Tasks {
        if task.Status == "in_progress" && task.Checkpoint != nil {
            recoverable = append(recoverable, *task)
        }
    }
    
    log.Printf("[StateMachine] Found %d tasks to recover", len(recoverable))
    
    return recoverable
}
func (sm *StateMachine) GetState() *SystemState {
    sm.mu.RLock()
    defer sm.mu.RUnlock()
    
    stateCopy := *sm.state
    return &stateCopy
}
func (sm *StateMachine) Subscribe(callback EventCallback) {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    
    sm.callbacks = append(sm.callbacks, callback)
}
