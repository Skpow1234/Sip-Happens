package config

// RedisRequiredForReadyz returns true when REDIS_REQUIRED=1 (fail /readyz if Redis is down).
func RedisRequiredForReadyz() bool {
	return getenvBoolDefault("REDIS_REQUIRED", false)
}
