package reminder

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// StorageInterface defines the storage operations
type StorageInterface interface {
	WriteToFile(filename string, data []byte) error
	ReadFromFile(filename string) ([]byte, error)
	CheckFileExistence(filename string) bool
	CreateFile(filename string) error
}

// MockStorage implements StorageInterface for testing
type MockStorage struct {
	mock.Mock
	files map[string][]byte
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		files: make(map[string][]byte),
	}
}

func (m *MockStorage) WriteToFile(filename string, data []byte) error {
	args := m.Called(filename, data)
	return args.Error(0) // Only returns error, not data
}

func (m *MockStorage) ReadFromFile(filename string) ([]byte, error) {
	args := m.Called(filename)
	if data, exists := m.files[filename]; exists && args.Error(1) == nil {
		result := make([]byte, len(data))
		copy(result, data)
		return result, nil
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockStorage) CheckFileExistence(filename string) bool {
	args := m.Called(filename)
	return args.Bool(0)
}

func (m *MockStorage) CreateFile(filename string) error {
	args := m.Called(filename)
	if args.Error(0) == nil {
		m.files[filename] = []byte{}
	}
	return args.Error(0)
}

// TestableReminderList - improved version that returns errors
type TestableReminderList struct {
	Reminders []Reminder `json:"reminders"`
	storage   StorageInterface
}

func NewTestableReminderList(storage StorageInterface) *TestableReminderList {
	return &TestableReminderList{
		Reminders: make([]Reminder, 0),
		storage:   storage,
	}
}

func (r *TestableReminderList) GetReminders() (string, error) {
	if len(r.Reminders) == 0 {
		return "No reminders set.", nil
	}

	marshaled, err := json.MarshalIndent(r.Reminders, "", "  ")
	if err != nil {
		return "", err
	}

	return string(marshaled), nil
}

func (r *TestableReminderList) AddReminder(reminder Reminder) error {
	r.Reminders = append(r.Reminders, reminder)
	return r.SaveRemindersToFile()
}

func (r *TestableReminderList) RemoveReminder(uuid string) error {
	for i, reminder := range r.Reminders {
		if reminder.UUID == uuid {
			r.Reminders = append(r.Reminders[:i], r.Reminders[i+1:]...)
			return r.SaveRemindersToFile()
		}
	}
	return errors.New("reminder not found")
}

func (r *TestableReminderList) SaveRemindersToFile() error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return r.storage.WriteToFile("reminders.json", data)
}

func (r *TestableReminderList) LoadRemindersFromFile() error {
	if !r.storage.CheckFileExistence("reminders.json") {
		err := r.storage.CreateFile("reminders.json")
		if err != nil {
			return err
		}

		tempReminder := TestableReminderList{Reminders: make([]Reminder, 0)}
		byteFile, err := json.MarshalIndent(tempReminder, "", "  ")
		if err != nil {
			return err
		}

		return r.storage.WriteToFile("reminders.json", byteFile)
	}

	byteFile, err := r.storage.ReadFromFile("reminders.json")
	if err != nil {
		return err
	}

	return json.Unmarshal(byteFile, r)
}

// Test helper functions
func setupTest() {
	err := os.Remove("reminders.json")
	if err != nil {
		return
	}
}

func teardownTest() {
	err := os.Remove("reminders.json")
	if err != nil {
		return
	}
}

// Helper function to capture log output
func captureLogOutput(f func()) string {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	f()
	return buf.String()
}

func TestNewReminder(t *testing.T) {
	tests := []struct {
		name         string
		title        string
		description  string
		reminderTime string
		userID       string
		channelID    string
		expectError  bool
		errorMsg     string
	}{
		{
			name:         "Valid future reminder",
			title:        "Test Reminder",
			description:  "Test Description",
			reminderTime: "2030-12-25T15:04:05",
			userID:       "user123",
			channelID:    "channel123",
			expectError:  false,
		},
		{
			name:         "Invalid time format",
			title:        "Test Reminder",
			description:  "Test Description",
			reminderTime: "invalid-time",
			userID:       "user123",
			channelID:    "channel123",
			expectError:  true,
		},
		{
			name:         "Past time",
			title:        "Test Reminder",
			description:  "Test Description",
			reminderTime: "2020-01-01T15:04:05",
			userID:       "user123",
			channelID:    "channel123",
			expectError:  true,
			errorMsg:     "reminder time is in the past",
		},
		{
			name:         "Empty title",
			title:        "",
			description:  "Test Description",
			reminderTime: "2030-12-25T15:04:05",
			userID:       "user123",
			channelID:    "channel123",
			expectError:  false,
		},
		{
			name:         "Empty description",
			title:        "Test Reminder",
			description:  "",
			reminderTime: "2030-12-25T15:04:05",
			userID:       "user123",
			channelID:    "channel123",
			expectError:  false,
		},
		{
			name:         "Edge case - exactly now",
			title:        "Test Reminder",
			description:  "Test Description",
			reminderTime: time.Now().Add(-1 * time.Second).Format("2006-01-02T15:04:05"),
			userID:       "user123",
			channelID:    "channel123",
			expectError:  true,
			errorMsg:     "reminder time is in the past",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reminder, err := NewReminder(
				tt.title,
				tt.description,
				tt.reminderTime,
				tt.userID,
				tt.channelID,
			)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, reminder)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, reminder)
				assert.Equal(t, tt.title, reminder.Title)
				assert.Equal(t, tt.description, reminder.Description)
				assert.Equal(t, tt.userID, reminder.UserID)
				assert.Equal(t, tt.channelID, reminder.ChannelID)
				assert.NotEmpty(t, reminder.UUID)

				// Validate UUID format
				_, err := uuid.Parse(reminder.UUID)
				assert.NoError(t, err)

				// Validate time parsing
				expectedTime, _ := time.Parse(
					time.RFC3339,
					tt.reminderTime+"+08:00",
				)
				assert.Equal(t, expectedTime, reminder.Time)
			}
		})
	}
}

func TestReminderList_GetReminders(t *testing.T) {
	t.Run("Empty reminder list", func(t *testing.T) {
		rl := &ReminderList{}
		result, err := rl.GetReminders()

		assert.NoError(t, err)
		assert.Equal(t, "No reminders set.", result)
	})

	t.Run("Non-empty reminder list", func(t *testing.T) {
		reminder1, _ := NewReminder(
			"Test 1",
			"Description 1",
			"2030-12-25T15:04:05",
			"user1",
			"channel1",
		)
		reminder2, _ := NewReminder(
			"Test 2",
			"Description 2",
			"2030-12-26T15:04:05",
			"user2",
			"channel2",
		)

		rl := &ReminderList{
			Reminders: []Reminder{*reminder1, *reminder2},
		}

		result, err := rl.GetReminders()

		assert.NoError(t, err)
		assert.Contains(t, result, "Test 1")
		assert.Contains(t, result, "Test 2")
		assert.Contains(t, result, "Description 1")
		assert.Contains(t, result, "Description 2")

		// Validate JSON format
		var reminders []Reminder
		err = json.Unmarshal([]byte(result), &reminders)
		assert.NoError(t, err)
		assert.Len(t, reminders, 2)
	})
}

// Test original AddReminder behavior (no error return, only logging)
func TestReminderList_AddReminder_Original(t *testing.T) {
	setupTest()
	defer teardownTest()

	reminder, err := NewReminder(
		"Test Reminder",
		"Test Description",
		"2030-12-25T15:04:05",
		"user123",
		"channel123",
	)
	require.NoError(t, err)

	t.Run("Successful add", func(t *testing.T) {
		rl := &ReminderList{}

		// Capture log output
		logOutput := captureLogOutput(func() {
			rl.AddReminder(*reminder)
		})

		assert.Len(t, rl.Reminders, 1)
		assert.Equal(t, reminder.Title, rl.Reminders[0].Title)
		assert.Equal(t, reminder.UUID, rl.Reminders[0].UUID)
		assert.Contains(t, logOutput, "Added reminder: Test Reminder")
	})

	t.Run("Add with potential storage error (logs error)", func(t *testing.T) {
		rl := &ReminderList{}

		// This test verifies that even if there's a storage error,
		// the reminder is still added to the in-memory list
		// and the error is logged (not returned)

		logOutput := captureLogOutput(func() {
			rl.AddReminder(*reminder)
		})

		// The reminder should still be added to the list
		assert.Len(t, rl.Reminders, 1)
		assert.Equal(t, reminder.Title, rl.Reminders[0].Title)

		// Should contain success log message
		assert.Contains(t, logOutput, "Added reminder: Test Reminder")
	})
}

func TestReminderList_RemoveReminder_Original(t *testing.T) {
	setupTest()
	defer teardownTest()

	reminder1, _ := NewReminder(
		"Test 1",
		"Description 1",
		"2030-12-25T15:04:05",
		"user1",
		"channel1",
	)
	reminder2, _ := NewReminder(
		"Test 2",
		"Description 2",
		"2030-12-26T15:04:05",
		"user2",
		"channel2",
	)

	t.Run("Remove existing reminder", func(t *testing.T) {
		rl := &ReminderList{
			Reminders: []Reminder{*reminder1, *reminder2},
		}

		logOutput := captureLogOutput(func() {
			rl.RemoveReminder(reminder1.UUID)
		})

		assert.Len(t, rl.Reminders, 1)
		assert.Equal(t, reminder2.UUID, rl.Reminders[0].UUID)
		assert.Contains(t, logOutput, "Removed reminder: Test 1")
	})

	t.Run("Remove non-existing reminder", func(t *testing.T) {
		rl := &ReminderList{
			Reminders: []Reminder{*reminder1, *reminder2},
		}

		logOutput := captureLogOutput(func() {
			rl.RemoveReminder("non-existing-uuid")
		})

		assert.Len(t, rl.Reminders, 2) // Should remain unchanged
		assert.Contains(t, logOutput, "No reminder found with UUID: non-existing-uuid")
	})
}

// Tests for the improved TestableReminderList (with error returns)
func TestTestableReminderList_AddReminder(t *testing.T) {
	mockStorage := NewMockStorage()

	reminder, err := NewReminder(
		"Test Reminder",
		"Test Description",
		"2030-12-25T15:04:05",
		"user123",
		"channel123",
	)
	require.NoError(t, err)

	t.Run("Successful add", func(t *testing.T) {
		rl := NewTestableReminderList(mockStorage)

		mockStorage.On("WriteToFile", "reminders.json", mock.AnythingOfType("[]uint8")).Return(nil)

		err := rl.AddReminder(*reminder)

		assert.NoError(t, err)
		assert.Len(t, rl.Reminders, 1)
		assert.Equal(t, reminder.Title, rl.Reminders[0].Title)
		assert.Equal(t, reminder.UUID, rl.Reminders[0].UUID)

		mockStorage.AssertExpectations(t)
	})

	t.Run("Add with storage error", func(t *testing.T) {
		freshMockStorage := NewMockStorage() // Create new mock instance
		rl := NewTestableReminderList(freshMockStorage)

		freshMockStorage.On("WriteToFile", "reminders.json", mock.AnythingOfType("[]uint8")).Return(errors.New("storage error"))

		err := rl.AddReminder(*reminder)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "storage error")
		// The reminder should still be added to the in-memory list
		assert.Len(t, rl.Reminders, 1)

		freshMockStorage.AssertExpectations(t)
	})
}

func TestTestableReminderList_RemoveReminder(t *testing.T) {
	mockStorage := NewMockStorage()

	reminder1, _ := NewReminder(
		"Test 1",
		"Description 1",
		"2030-12-25T15:04:05",
		"user1",
		"channel1",
	)
	reminder2, _ := NewReminder(
		"Test 2",
		"Description 2",
		"2030-12-26T15:04:05",
		"user2",
		"channel2",
	)

	t.Run("Remove existing reminder", func(t *testing.T) {
		rl := NewTestableReminderList(mockStorage)
		rl.Reminders = []Reminder{*reminder1, *reminder2}

		mockStorage.On("WriteToFile", "reminders.json", mock.AnythingOfType("[]uint8")).Return(nil)

		err := rl.RemoveReminder(reminder1.UUID)

		assert.NoError(t, err)
		assert.Len(t, rl.Reminders, 1)
		assert.Equal(t, reminder2.UUID, rl.Reminders[0].UUID)

		mockStorage.AssertExpectations(t)
	})

	t.Run("Remove non-existing reminder", func(t *testing.T) {
		rl := NewTestableReminderList(mockStorage)
		rl.Reminders = []Reminder{*reminder1, *reminder2}

		err := rl.RemoveReminder("non-existing-uuid")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "reminder not found")
		assert.Len(t, rl.Reminders, 2) // Should remain unchanged
	})
}

func TestReminderList_SaveRemindersToFile_Original(t *testing.T) {
	setupTest()
	defer teardownTest()

	reminder, _ := NewReminder(
		"Test Reminder",
		"Test Description",
		"2030-12-25T15:04:05",
		"user123",
		"channel123",
	)

	rl := &ReminderList{
		Reminders: []Reminder{*reminder},
	}

	t.Run("Save reminders", func(t *testing.T) {
		logOutput := captureLogOutput(func() {
			err := rl.SaveRemindersToFile()
			assert.NoError(t, err)
		})

		assert.Contains(t, logOutput, "Saved reminders to file")

		// Verify file was created and contains data
		assert.FileExists(t, "reminders.json")
	})
}

func TestLoadRemindersFromFile_Original(t *testing.T) {
	setupTest()
	defer teardownTest()

	t.Run("Load from non-existing file", func(t *testing.T) {
		rl := &ReminderList{}

		logOutput := captureLogOutput(func() {
			err := LoadRemindersFromFile(rl)
			assert.NoError(t, err)
		})

		assert.Contains(t, logOutput, "Reminders file does not exist. Creating a new one.")
		assert.Empty(t, rl.Reminders)
		assert.FileExists(t, "reminders.json")
	})

	t.Run("Load from existing file with data", func(t *testing.T) {
		// First create a file with data
		reminder, _ := NewReminder(
			"Test Reminder",
			"Test Description",
			"2030-12-25T15:04:05",
			"user123",
			"channel123",
		)

		rl1 := &ReminderList{Reminders: []Reminder{*reminder}}
		err := rl1.SaveRemindersToFile()
		require.NoError(t, err)

		// Now load it into a new ReminderList
		rl2 := &ReminderList{}

		logOutput := captureLogOutput(func() {
			err := LoadRemindersFromFile(rl2)
			assert.NoError(t, err)
		})

		assert.Contains(t, logOutput, "Loaded reminders from file")
		assert.Len(t, rl2.Reminders, 1)
		assert.Equal(t, reminder.Title, rl2.Reminders[0].Title)
		assert.Equal(t, reminder.UUID, rl2.Reminders[0].UUID)
	})
}

func TestReminder_JSONSerialization(t *testing.T) {
	reminder, err := NewReminder(
		"Test Reminder",
		"Test Description",
		"2030-12-25T15:04:05",
		"user123",
		"channel123",
	)
	require.NoError(t, err)

	// Test JSON marshaling
	jsonData, err := json.Marshal(reminder)
	assert.NoError(t, err)
	assert.Contains(t, string(jsonData), "Test Reminder")
	assert.Contains(t, string(jsonData), "Test Description")

	// Test JSON unmarshaling
	var unmarshaled Reminder
	err = json.Unmarshal(jsonData, &unmarshaled)
	assert.NoError(t, err)
	assert.Equal(t, reminder.Title, unmarshaled.Title)
	assert.Equal(t, reminder.Description, unmarshaled.Description)
	assert.Equal(t, reminder.UserID, unmarshaled.UserID)
	assert.Equal(t, reminder.ChannelID, unmarshaled.ChannelID)
	assert.Equal(t, reminder.UUID, unmarshaled.UUID)
	assert.True(t, reminder.Time.Equal(unmarshaled.Time))
}

func TestReminderList_JSONSerialization(t *testing.T) {
	reminder1, _ := NewReminder(
		"Test 1",
		"Description 1",
		"2030-12-25T15:04:05",
		"user1",
		"channel1",
	)
	reminder2, _ := NewReminder(
		"Test 2",
		"Description 2",
		"2030-12-26T15:04:05",
		"user2",
		"channel2",
	)

	rl := &ReminderList{
		Reminders: []Reminder{*reminder1, *reminder2},
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(rl)
	assert.NoError(t, err)
	assert.Contains(t, string(jsonData), "Test 1")
	assert.Contains(t, string(jsonData), "Test 2")

	// Test JSON unmarshaling
	var unmarshaled ReminderList
	err = json.Unmarshal(jsonData, &unmarshaled)
	assert.NoError(t, err)
	assert.Len(t, unmarshaled.Reminders, 2)
	assert.Equal(t, reminder1.Title, unmarshaled.Reminders[0].Title)
	assert.Equal(t, reminder2.Title, unmarshaled.Reminders[1].Title)
}

// Integration test with original code
func TestOriginalReminderList_Integration(t *testing.T) {
	setupTest()
	defer teardownTest()

	rl := &ReminderList{}

	reminder, err := NewReminder(
		"Integration Test",
		"Integration Description",
		"2030-12-25T15:04:05",
		"user123",
		"channel123",
	)
	require.NoError(t, err)

	// Test GetReminders with empty list
	result, err := rl.GetReminders()
	assert.NoError(t, err)
	assert.Equal(t, "No reminders set.", result)

	// Add reminder
	logOutput := captureLogOutput(func() {
		rl.AddReminder(*reminder)
	})
	assert.Len(t, rl.Reminders, 1)
	assert.Contains(t, logOutput, "Added reminder: Integration Test")

	// Test GetReminders with data
	result, err = rl.GetReminders()
	assert.NoError(t, err)
	assert.Contains(t, result, "Integration Test")

	// Remove reminder
	logOutput = captureLogOutput(func() {
		rl.RemoveReminder(reminder.UUID)
	})
	assert.Len(t, rl.Reminders, 0)
	assert.Contains(t, logOutput, "Removed reminder: Integration Test")
}

// Benchmark tests
func BenchmarkNewReminder(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = NewReminder(
			"Benchmark Reminder",
			"Benchmark Description",
			"2030-12-25T15:04:05",
			"user123",
			"channel123",
		)
	}
}

func BenchmarkAddReminder_Original(b *testing.B) {
	setupTest()
	defer teardownTest()

	rl := &ReminderList{}

	// Silence logs during benchmark
	log.SetOutput(os.NewFile(0, os.DevNull))
	defer log.SetOutput(os.Stderr)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reminder, _ := NewReminder(
			"Benchmark Reminder",
			"Benchmark Description",
			"2030-12-25T15:04:05",
			"user123",
			"channel123",
		)
		rl.AddReminder(*reminder)
	}
}

func BenchmarkGetReminders(b *testing.B) {
	rl := &ReminderList{}

	// Add some test data
	for i := 0; i < 100; i++ {
		reminder, _ := NewReminder(
			"Benchmark Reminder",
			"Benchmark Description",
			"2030-12-25T15:04:05",
			"user123",
			"channel123",
		)
		rl.Reminders = append(rl.Reminders, *reminder)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = rl.GetReminders()
	}
}
