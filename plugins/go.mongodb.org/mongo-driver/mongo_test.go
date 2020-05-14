// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-2019 Datadog, Inc.
// Copyright 2020 The OpenTelemetry Authors

package mongo

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	mocktracer "go.opentelemetry.io/contrib/internal/trace"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	_, ok := os.LookupEnv("INTEGRATION")
	if !ok {
		fmt.Println("--- SKIP: to enable integration test, set the INTEGRATION environment variable")
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func Test(t *testing.T) {
	mt := mocktracer.NewTracer("mongodb")

	hostname, port := "localhost", "27017"

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	ctx, span := mt.Start(ctx, "mongodb-test")

	addr := fmt.Sprintf("mongodb://localhost:27017/?connect=direct")
	opts := options.Client()
	opts.Monitor = NewMonitor(WithTracer(mt))
	opts.ApplyURI(addr)
	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Database("test-database").Collection("test-collection").InsertOne(ctx, bson.D{{Key: "test-item", Value: "test-value"}})
	if err != nil {
		t.Fatal(err)
	}

	span.End()

	spans := mt.EndedSpans()
	assert.Len(t, spans, 2)
	assert.Equal(t, spans[0].SpanContext().TraceID, spans[1].SpanContext().TraceID)

	s := spans[0]
	assert.Equal(t, "mongo", s.Attributes[ServiceNameKey].AsString())
	assert.Equal(t, "mongo.insert", s.Attributes[ResourceNameKey].AsString())
	assert.Equal(t, hostname, s.Attributes[PeerHostnameKey].AsString())
	assert.Equal(t, port, s.Attributes[PeerPortKey].AsString())
	assert.Contains(t, s.Attributes[DBStatementKey].AsString(), `"test-item":"test-value"`)
	assert.Equal(t, "test-database", s.Attributes[DBInstanceKey].AsString())
	assert.Equal(t, "mongo", s.Attributes[DBTypeKey].AsString())
}
