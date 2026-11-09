package launch

import "context"

// Test hooks for the external test package.
func SniffImage(data []byte) (string, error)  { return sniffImage(data) }
func BlueskyCopy(data []byte) ([]byte, error) { return blueskyCopy(context.Background(), data) }
func Orientation(data []byte) int             { return orientation(data) }
