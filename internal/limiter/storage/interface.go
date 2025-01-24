package storage

import (
    "context"
    "time"
)

// StorageBackend defines the interface for storing rate-limiting data.
type StorageBackend interface {
    // Increment increments the count for a key and returns the new count.
    // If the key doesn't exist, it is created with an expiration time.
    Increment(ctx context.Context, key string, expiration time.Duration) (int64, error)

    // Get returns the current count and expiration time for a key.
    Get(ctx context.Context, key string) (count int64, expiration time.Time, err error)

    // SetIfNotExists sets the key to a value if it doesn't exist, with an expiration time.
    SetIfNotExists(ctx context.Context, key string, value int64, expiration time.Duration) (bool, error)

    // Delete removes a key from storage.
    Delete(ctx context.Context, key string) error

    // Expire updates the expiration time for a key.
    Expire(ctx context.Context, key string, expiration time.Duration) error
}


type Storage interface {  
    // Atomically:  
    // 1. Trim entries older than `windowStart` for `key`.  
    // 2. Check if current count < `limit`.  
    // 3. If allowed, add `timestamp` to the key's set.  
    // Returns whether the request is allowed or an error.  
    CheckAndAdd(  
        ctx context.Context,  
        key string,  
        windowStart time.Time,  
        timestamp time.Time,  
        limit int,  
    ) (bool, error)

    GetCount(ctx context.Context, key string, windowStart time.Time) (int, error)  
    GetOldestTimestamp(ctx context.Context, key string, windowStart time.Time) (time.Time, error)  
    ResetKey(ctx context.Context, key string) error
}