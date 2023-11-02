package main

import (
	"fmt"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/tidepool-org/platform/log"
)

// zapPlatformAdapter implements log.Logger for use with a zap.SugaredLogger.
type zapPlatformAdapter struct {
	zapper *zap.SugaredLogger
	mu     sync.Mutex
}

func NewZapPlatformAdapter(zapper *zap.SugaredLogger) *zapPlatformAdapter {
	return &zapPlatformAdapter{
		zapper: zapper,
	}
}

func (a *zapPlatformAdapter) Logf(level log.Level, message string, args ...interface{}) {
	formatted := fmt.Sprintf(message, args...)
	a.Log(level, formatted)
}

var loggerToZapLevels = map[log.Level]zapcore.Level{
	log.DebugLevel: zapcore.DebugLevel,
	log.InfoLevel:  zapcore.InfoLevel,
	log.WarnLevel:  zapcore.WarnLevel,
	log.ErrorLevel: zapcore.ErrorLevel,
}

func (a *zapPlatformAdapter) Log(level log.Level, message string) {
	if zapLevel, found := loggerToZapLevels[level]; found {
		a.addOne().zapper.Desugar().Log(zapLevel, message)
		return
	}
	a.zapper.Debugf("zapPlatformHandler: unhandled log.Level %q", level)
	a.zapper.With("log.level", level).Info(message)
}

func (a *zapPlatformAdapter) addOne() *zapPlatformAdapter {
	return NewZapPlatformAdapter(a.zapper.Desugar().WithOptions(zap.AddCallerSkip(1)).Sugar())
}

func (a *zapPlatformAdapter) Debug(message string) {
	a.addOne().Log(log.DebugLevel, message)
}

func (a *zapPlatformAdapter) Info(message string) {
	a.addOne().Log(log.InfoLevel, message)
}

func (a *zapPlatformAdapter) Warn(message string) {
	a.addOne().Log(log.WarnLevel, message)
}

func (a *zapPlatformAdapter) Error(message string) {
	a.addOne().Log(log.ErrorLevel, message)
}

func (a *zapPlatformAdapter) Debugf(message string, args ...interface{}) {
	a.addOne().Logf(log.DebugLevel, message, args...)
}

func (a *zapPlatformAdapter) Infof(message string, args ...interface{}) {
	a.addOne().Logf(log.InfoLevel, message, args...)
}

func (a *zapPlatformAdapter) Warnf(message string, args ...interface{}) {
	a.addOne().Logf(log.WarnLevel, message, args...)
}

func (a *zapPlatformAdapter) Errorf(message string, args ...interface{}) {
	a.addOne().Logf(log.ErrorLevel, message, args...)
}

func (a *zapPlatformAdapter) WithError(err error) log.Logger {
	return NewZapPlatformAdapter(a.zapper.With(zap.Error(err)))
}

func (a *zapPlatformAdapter) WithField(key string, value interface{}) log.Logger {
	return NewZapPlatformAdapter(a.zapper.With(key, value))
}

func (a *zapPlatformAdapter) WithFields(fields log.Fields) log.Logger {
	c := a.zapper
	for key, value := range fields {
		c = c.With(key, value)
	}
	return NewZapPlatformAdapter(c)
}

func (a *zapPlatformAdapter) WithLevelRank(level log.Level, rank log.Rank) log.Logger {
	// There are no docs for LevelRanks, and it's not obvious what effect it
	// has, so just skipping for now.
	a.zapper.Debugf("zapPlatformAdapter: unimplemented method: WithLevelRank")
	return a
}

func (a *zapPlatformAdapter) WithLevelRanks(levelRanks log.LevelRanks) log.Logger {
	// There are no docs for LevelRanks, and it's not obvious what effect it
	// has, so just skipping for now.
	a.zapper.Debugf("zapPlatformAdapter: unimplemented method: WithLevelRanks")
	return a
}

func (a *zapPlatformAdapter) WithLevel(level log.Level) log.Logger {
	lvl, err := zapcore.ParseLevel(string(level))
	if err != nil {
		return a
	}
	return NewZapPlatformAdapter(a.zapper.WithOptions(zap.IncreaseLevel(lvl)))
}

func (a *zapPlatformAdapter) Level() log.Level {
	a.zapper.Debugf("zapPlatformAdapter: unimplemented method: Level")
	// I don't see a way to retrieve this infromation from a zap logger, and I
	// don't see any code that calls this method anyway.
	return log.DebugLevel
}
