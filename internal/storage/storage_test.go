package storage

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	filename = "filename"
)

func ExampleStorage() {
	ctx, cancel := context.WithCancel(context.Background())

	storage := New(ctx, WithPersistent(Persistent{}), WithJanitor(Janitor{}))

	storage.Put("test", []byte(`{"key": "value"}`), nil)

	got, ok := storage.Get("test")
	if ok {
		fmt.Println(string(got))
	}

	cancel()
	storage.Shutdown()

	loadedStorage := New(context.Background(), WithPersistent(Persistent{
		Name:     "go-persista",
		Format:   GobFormat,
		Interval: 5 * time.Second,
	}))

	got, ok = loadedStorage.Get("test")
	if ok {
		fmt.Println(string(got))
	}

	// Output:
	// {"key": "value"}
	// {"key": "value"}
}

func TestStorageGetExpired(t *testing.T) {
	storage := New(context.Background())

	data := []byte(`{"key": "value"}`)
	tm := time.Now().Add(-1 * time.Second)

	storage.Put("test", data, &tm)

	_, ok := storage.Get("test")
	assert.False(t, ok)
}

func TestStorageSaveOnShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	storage := New(ctx, WithPersistent(Persistent{
		Name:     filename,
		Format:   GobFormat,
		Interval: 1 * time.Hour,
	}))

	data := []byte(`{"key": "value"}`)

	storage.Put("test", data, nil)

	got, ok := storage.Get("test")
	assert.True(t, ok)
	assert.Equal(t, data, got)

	cancel()
	storage.Shutdown()

	loadedStorage := New(context.Background(), WithPersistent(Persistent{
		Name:     filename,
		Format:   GobFormat,
		Interval: 1 * time.Hour,
	}))

	got, ok = loadedStorage.Get("test")
	assert.True(t, ok)
	assert.Equal(t, data, got)

	os.Remove("filename." + string(GobFormat))
}

func TestStorageJanitor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	storage := New(ctx, WithJanitor(Janitor{
		Interval: 100 * time.Millisecond,
	}))

	tm := time.Now().Add(50 * time.Millisecond)

	storage.Put("test", []byte(`{"key": "value"}`), &tm)

	time.Sleep(100 * time.Millisecond)

	// janitor should remove expired object
	storage.mu.RLock()
	_, ok := storage.storage["test"]
	storage.mu.RUnlock()
	assert.False(t, ok)

	cancel()
	storage.Shutdown()
}

func TestStorageSaveAndLoad(t *testing.T) {
	testCases := []struct {
		name   string
		format Format
	}{
		{"JSON format", JSONFormat},
		{"Gob format", GobFormat},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			storage := &Storage{storage: make(map[string]Object)}
			testKey := "test"
			testData := []byte(`{"key": "value"}`)
			testExpires := time.Now().Add(24 * time.Hour)
			testName := "filename"

			storage.storage[testKey] = Object{Data: testData, Expires: &testExpires}

			cnt, err := storage.save(testName, tc.format)
			assert.Equal(t, 1, cnt)
			require.NoError(t, err)

			loadedStorage := &Storage{storage: make(map[string]Object)}
			cnt, err = loadedStorage.load(testName)
			assert.Equal(t, 1, cnt)
			require.NoError(t, err)

			assert.Equal(t, 1, len(loadedStorage.storage))
			assert.Equal(t, testData, loadedStorage.storage[testKey].Data)
			assert.WithinDuration(t, testExpires, *loadedStorage.storage[testKey].Expires, time.Second)

			os.Remove(testName + "." + string(tc.format))
		})
	}
}

func BenchmarkStorage_saveJson(b *testing.B) {
	st := New(context.Background())

	for i := 0; i < 50000; i++ {
		st.Put(fmt.Sprintf("key-%d", i), []byte(fmt.Sprintf(`{"key": "value-%d"}`, i)), nil)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = st.save("filename", JSONFormat)
	}

	os.Remove("filename." + string(JSONFormat))
}

func BenchmarkStorage_saveGob(b *testing.B) {
	st := New(context.Background())

	for i := 0; i < 50000; i++ {
		st.Put(fmt.Sprintf("key-%d", i), []byte(fmt.Sprintf(`{"key": "value-%d"}`, i)), nil)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = st.save("filename", GobFormat)
	}

	os.Remove("filename." + string(GobFormat))
}
