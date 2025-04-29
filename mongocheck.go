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

// 工具的参数
var (
	src = flag.String("src", "mongodb://testUser:testPwd@localhost:27017/admin?authSource=admin", "源集群地址, 必填")
	// dst := flag.String("dst", "mongodb://testUser:testPwd@localhost:27018/admin?authSource=admin", "源集群地址, 必填")
	db = flag.String("db", "testdb", "要检查的数据库名, 必填")
	// count := flag.Int("count", 100, "每个表要抽样检查的数据条数")
	// percent := flag.Int("percent", 10, "每个表要抽样检查的数据百分比,如果同时指定了count,则取两者的最小值")
)

func CheckCollection(srcColl *mongo.Collection, dstColl *mongo.Collection) {
	// 先对比文档数
	srcCount, err := srcColl.EstimatedDocumentCount(context.Background())
	if err != nil {
		log.Fatalf("获取源集合 %s 文档数失败: %v", srcColl.Name(), err)
	}

	dstCount, err := dstColl.EstimatedDocumentCount(context.Background())
	if err != nil {
		log.Fatalf("获取目标集合 %s 文档数失败: %v", dstColl.Name(), err)
	}

	log.Println("源集群文档数", srcCount, "目标集群文档数", dstCount)

	// 抽样数据对比,
	// 这里不使用 $sample 抽样, 因为数据量大的时候会走 top-k 排序，资源消耗大
}

func main() {
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
