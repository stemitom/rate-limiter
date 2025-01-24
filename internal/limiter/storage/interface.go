package storage

import (
    "context"
    "time"
)

// Storage defines the interface for storing rate-limiting data.
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