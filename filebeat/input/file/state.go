package file

import (
	"os"
	"sync"
	"time"

	"github.com/elastic/beats/libbeat/logp"
)

// State is used to communicate the reading state of a file
type State struct {
	Source      string      `json:"source"`
	Offset      int64       `json:"offset"`
	Finished    bool        `json:"-"` // harvester state
	Fileinfo    os.FileInfo `json:"-"` // the file info
	FileStateOS StateOS
	LastSeen    time.Time `json:"last_seen"`
}

// NewState creates a new file state
func NewState(fileInfo os.FileInfo, path string) State {
	return State{
		Fileinfo:    fileInfo,
		Source:      path,
		Finished:    false,
		FileStateOS: GetOSState(fileInfo),
		LastSeen:    time.Now(),
	}
}

// States handles list of FileState
type States struct {
	states []State
	mutex  sync.Mutex
}

func NewStates() *States {
	return &States{
		states: []State{},
	}
}

// Update updates a state. If previous state didn't exist, new one is created
func (s *States) Update(newState State) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	index, _ := s.findPrevious(newState)
	newState.LastSeen = time.Now()

	if index >= 0 {
		s.states[index] = newState
	} else {
		// No existing state found, add new one
		s.states = append(s.states, newState)
		logp.Debug("prospector", "New state added for %s", newState.Source)
	}
}

func (s *States) FindPrevious(newState State) (int, State) {
	// TODO: This currently blocks writing updates every time state is fetched. Should be improved for performance
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.findPrevious(newState)
}

// findPreviousState returns the previous state fo the file
// In case no previous state exists, index -1 is returned
func (s *States) findPrevious(newState State) (int, State) {

	// TODO: This could be made potentially more performance by using an index (harvester id) and only use iteration as fall back
	for index, oldState := range s.states {
		// This is using the FileStateOS for comparison as FileInfo identifiers can only be fetched for existing files
		if oldState.FileStateOS.IsSame(newState.FileStateOS) {
			return index, oldState
		}
	}

	return -1, State{}
}

// Cleanup cleans up the state array. All states which are older then `older` are removed
func (s *States) Cleanup(older time.Duration) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for i, state := range s.states {

		// File wasn't seen for longer then older -> remove state
		if time.Since(state.LastSeen) > older {
			logp.Debug("prospector", "State removed for %s because of older: %s", state.Source)
			s.states = append(s.states[:i], s.states[i+1:]...)
		}
	}

}

// Count returns number of states
func (s *States) Count() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return len(s.states)
}

// Returns a copy of the file states
func (s *States) GetStates() []State {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	newStates := make([]State, len(s.states))
	copy(newStates, s.states)

	return newStates
}

// SetStates overwrites all internal states with the given states array
func (s *States) SetStates(states []State) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.states = states
}

// Copy create a new copy of the states object
func (s *States) Copy() *States {
	states := NewStates()
	states.states = s.GetStates()
	return states
}
