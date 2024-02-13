// Package sdk is the official AWS SDK v2 for the Go programming language.
//
// aws-sdk-go-v2 is the the v2 of the AWS SDK for the Go programming language.
//
// # Getting started
//
// The best way to get started working with the SDK is to use `go get` to add the
// SDK and desired service clients to your Go dependencies explicitly.
//
//	go get github.com/aws/aws-sdk-go-v2
//	go get github.com/aws/aws-sdk-go-v2/config
//	go get github.com/aws/aws-sdk-go-v2/service/dynamodb
//
// # Hello AWS
//
// This example shows how you can use the v2 SDK to make an API request using the
// SDK's Amazon DynamoDB client.
//
//	package main
//
//	import (
//	    "context"
//	    "fmt"
//	    "log"
//
//	    "github.com/aws/aws-sdk-go-v2/aws"
//	    "github.com/aws/aws-sdk-go-v2/config"
//	    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
//	)
//
//	func main() {
//	    // Using the SDK's default configuration, loading additional config
//	    // and credentials values from the environment variables, shared
//	    // credentials, and shared configuration files
//	    cfg, err := config.LoadDefaultConfig(context.TODO(),
//	   		config.WithRegion("us-west-2"),
//	   	)
//	    if err != nil {
//	        log.Fatalf("unable to load SDK config, %v", err)
//	    }
//
//	    // Using the Config value, create the DynamoDB client
//	    svc := dynamodb.NewFromConfig(cfg)
//
//	    // Build the request with its input parameters
//	    resp, err := svc.ListTables(context.TODO(), &dynamodb.ListTablesInput{
//	        Limit: aws.Int32(5),
//	    })
//	    if err != nil {
//	        log.Fatalf("failed to list tables, %v", err)
//	    }
//
//	    fmt.Println("Tables:")
//	    for _, tableName := range resp.TableNames {
//	        fmt.Println(tableName)
//	    }
//	}
package sdk
