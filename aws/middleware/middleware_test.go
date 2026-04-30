package middleware_test

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	awsmiddleware "github.com/ellogroup/ello-golang-otel/aws/middleware"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAppendToConfig_AddsMiddlewares(t *testing.T) {
	cfg := aws.Config{}
	before := len(cfg.APIOptions)

	awsmiddleware.AppendToConfig(&cfg)

	assert.Greater(t, len(cfg.APIOptions), before)
}
