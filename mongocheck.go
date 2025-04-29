package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	src := flag.String("src", "mongodb://testUser:testPwd@localhost:27017/admin?authSource=admin", "源集群地址, 必填")
	// dst := flag.String("dst", "mongodb://testUser:testPwd@localhost:27018/admin?authSource=admin", "源集群地址, 必填")
	db := flag.String("db", "testdb", "要检查的数据库名, 必填")
	// count := flag.Int("count", 100, "每个表要抽样检查的数据条数")
	// percent := flag.Int("percent", 10, "每个表要抽样检查的数据百分比,如果同时指定了count,则取两者的最小值")

	flag.Parse()

	// 连接超时时间
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	srcClient, err := mongo.Connect(ctx, options.Client().ApplyURI(*src))
	if err != nil {
		log.Fatalf("连接失败: %v", err)
	}
	defer func() {
		if err = srcClient.Disconnect(ctx); err != nil {
			log.Fatal(err)
		}
	}()

	// 5. 获取数据库对象, 并展示所有集合
	database := srcClient.Database(*db)
	collections, err := database.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		log.Fatalf("获取集合失败: %v", err)
	}
	fmt.Printf("数据库 %s 包含 %d 个集合:\n", *db, len(collections))
	for _, collName := range collections {
		coll := database.Collection(collName)
		count, err := coll.EstimatedDocumentCount(ctx)
		if err != nil {
			log.Fatalf("获取集合 %s 文档数失败: %v", collName, err)
		}
		fmt.Println("-", collName, "文档数:", count)
	}
}
