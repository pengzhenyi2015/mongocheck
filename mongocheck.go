package main

import (
	"context"
	"flag"
	"log"
	"math"
	"math/rand"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 工具的参数
var (
	src     = flag.String("src", "mongodb://testUser:testPwd@localhost:27017/admin?authSource=admin", "源集群地址, 必填")
	dst     = flag.String("dst", "mongodb://testUser:testPwd@localhost:27018/admin?authSource=admin", "源集群地址, 必填")
	db      = flag.String("db", "", "要检查的数据库名, 必填")
	coll    = flag.String("coll", "", "要检查的集合名, 可选, 如果不指定则检查所有集合")
	count   = flag.Int("count", 100, "每个表要抽样检查的数据条数")
	percent = flag.Int("percent", 10, "每个表要抽样检查的数据百分比,如果同时指定了count,则取两者的最小值")
)

func checkCollection(srcColl *mongo.Collection, dstColl *mongo.Collection) {
	// 先对比文档数
	srcCount, err := srcColl.EstimatedDocumentCount(context.Background())
	if err != nil {
		log.Fatalf("获取源集合 %s 文档数失败: %v", srcColl.Name(), err)
	}

	dstCount, err := dstColl.EstimatedDocumentCount(context.Background())
	if err != nil {
		log.Fatalf("获取目标集合 %s 文档数失败: %v", dstColl.Name(), err)
	}

	// 抽样数据对比,
	// 不使用 $sample 抽样, 因为数据量大的时候会走 top-k 排序，资源消耗大
	// 确定一个随机起始点，然后确定好平均步长后抽样数据
	sampleSize := int64(math.Min(float64(*count), float64(srcCount)*float64(*percent)/100))
	stepSize := srcCount / sampleSize
	startIndex := int64(rand.Float64() * float64(srcCount%sampleSize))
	log.Printf("开始比对集合:%s, 源集群文档数:%d, 目标集群文档数:%d, 抽样数据条数:%d, 随机起始位置:%d, 步长:%d",
		srcColl.Name(), srcCount, dstCount, sampleSize, startIndex, stepSize)
}

func hasDatabase(client *mongo.Client, dbName string) bool {
	dbNames, err := client.ListDatabaseNames(context.Background(), bson.M{})
	if err != nil {
		log.Fatalf("获取数据库列表失败: %v", err)
	}

	for _, name := range dbNames {
		if name == dbName {
			return true
		}
	}
	return false
}

func hasCollection(db *mongo.Database, collName string) bool {
	collNames, err := db.ListCollectionNames(context.Background(), bson.M{})
	if err != nil {
		log.Fatalf("获取集合列表失败: %v", err)
	}

	for _, name := range collNames {
		if name == collName {
			return true
		}
	}
	return false
}

func main() {
	flag.Parse()

	if *src == "" || *dst == "" || *db == "" {
		flag.Usage()
		log.Fatalln("请输入合法的参数， src/dst/db 参数不能为空")
	}

	// 连接超时时间
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	srcClient, err := mongo.Connect(ctx, options.Client().ApplyURI(*src))
	if err != nil {
		log.Fatalf("源集群连接失败: %v", err)
	}
	defer func() {
		if err = srcClient.Disconnect(ctx); err != nil {
			log.Fatal(err)
		}
	}()

	dstClient, err := mongo.Connect(ctx, options.Client().ApplyURI(*dst))
	if err != nil {
		log.Fatalf("目标集群连接失败: %v", err)
	}
	defer func() {
		if err = dstClient.Disconnect(ctx); err != nil {
			log.Fatal(err)
		}
	}()

	// 检查 db 是否存在
	if !hasDatabase(srcClient, *db) {
		log.Fatalf("源集群数据库 %s 不存在", *db)
	}
	if !hasDatabase(dstClient, *db) {
		log.Fatalf("目标集群数据库 %s 不存在", *db)
	}

	srcDB := srcClient.Database(*db)
	dstDB := dstClient.Database(*db)

	if *coll != "" {
		if !hasCollection(srcDB, *coll) {
			log.Fatalf("源集群集合 %s 不存在", *coll)
		}
		if !hasCollection(dstDB, *coll) {
			log.Fatalf("目标集群集合 %s 不存在", *coll)
		}
		checkCollection(srcDB.Collection(*coll), dstDB.Collection(*coll))
		return
	}

	srcColls, err := srcDB.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		log.Fatalf("源集群获取集合列表失败: %v", err)
	}
	for _, collName := range srcColls {
		if !hasCollection(dstDB, collName) {
			log.Fatalf("目标集群集合 %s 不存在", collName)
		}
		checkCollection(srcDB.Collection(collName), dstDB.Collection(collName))
	}
}
