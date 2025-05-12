package main

import (
	"bytes"
	"context"
	"flag"
	"log"
	"math"
	"math/rand"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 工具的参数
var (
	src              = flag.String("src", "mongodb://testUser:testPwd@localhost:27017/admin?authSource=admin", "源集群地址, 必填")
	dst              = flag.String("dst", "mongodb://testUser:testPwd@localhost:27018/admin?authSource=admin", "源集群地址, 必填")
	db               = flag.String("db", "", "要检查的数据库名, 必填")
	coll             = flag.String("coll", "", "要检查的集合名, 可选, 如果不指定则检查所有集合")
	count            = flag.Int("count", 100, "每个表要抽样检查的数据条数")
	percent          = flag.Int("percent", 10, "每个表要抽样检查的数据百分比,如果同时指定了count,则取两者的最小值")
	checkIndex       = flag.Bool("checkIndex", false, "是否比对索引")
	continueNotExist = flag.Bool("continueNotExist", false, "目标集群有数据不存在时,是否报错继续检查。一般目标集群一直处于增量同步的情况下考虑使用")
)

func checkIndexes(srcColl *mongo.Collection, dstColl *mongo.Collection) {
	srcIndexesCursor, err := srcColl.Indexes().List(context.Background())
	if err != nil {
		log.Fatalf("获取源集合 %s 索引失败: %v", srcColl.Name(), err)
	}
	dstIndexesCursor, err := dstColl.Indexes().List(context.Background())
	if err != nil {
		log.Fatalf("获取目标集合 %s 索引失败: %v", dstColl.Name(), err)
	}

	srcIndexList := make([]bson.Raw, 0)
	for srcIndexesCursor.Next(context.Background()) {
		srcIndexList = append(srcIndexList, srcIndexesCursor.Current)
	}
	dstIndexList := make([]bson.Raw, 0)
	for dstIndexesCursor.Next(context.Background()) {
		dstIndexList = append(dstIndexList, dstIndexesCursor.Current)
	}
	sort.Slice(srcIndexList, func(i, j int) bool {
		return bytes.Compare(srcIndexList[i], srcIndexList[j]) < 0
	})
	sort.Slice(dstIndexList, func(i, j int) bool {
		return bytes.Compare(dstIndexList[i], dstIndexList[j]) < 0
	})

	if len(srcIndexList) != len(dstIndexList) {
		log.Fatalf("源集合 %s 和目标集合 %s 索引数量不一致, 源:%d, 目标:%d", srcColl.Name(), dstColl.Name(), len(srcIndexList), len(dstIndexList))
	}

	for i := range srcIndexList {
		if !bytes.Equal(srcIndexList[i], dstIndexList[i]) {
			log.Fatalf("源集合 %s 和目标集合 %s 索引不一致, 源:%v, 目标:%v", srcColl.Name(), dstColl.Name(), srcIndexList[i].String(), dstIndexList[i].String())
		}
	}

	log.Printf("源集合 %s 和目标集合 %s 索引一致", srcColl.Name(), dstColl.Name())
}

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
	currentIndex := int64(rand.Float64() * float64(srcCount%sampleSize))
	log.Printf("开始比对集合:%s, 源集群文档数:%d, 目标集群文档数:%d, 抽样数据条数:%d, 随机起始位置:%d, 步长:%d",
		srcColl.Name(), srcCount, dstCount, sampleSize, currentIndex, stepSize)

	// 先比对第一条数据
	timeout := time.Minute
	findOneOptions := options.FindOneOptions{
		MaxTime: &timeout,
		Sort:    bson.D{{Key: "_id", Value: 1}},
		Skip:    &currentIndex,
	}
	srcDoc, err := srcColl.FindOne(context.Background(), bson.M{}, &findOneOptions).Raw()
	if err != nil {
		log.Fatalf("获取源集合 %s 第 %d 条数据失败: %v", srcColl.Name(), currentIndex, err)
	}
	id := srcDoc.Lookup("_id")

	dstDoc, err := dstColl.FindOne(context.Background(), bson.M{"_id": id}).Raw()
	if err != nil {
		log.Fatalf("获取目标集合 %s 对应的数据失败, _id:%v, err: %v", srcColl.Name(), id.String(), err)
	}
	if !bytes.Equal(srcDoc, dstDoc) {
		log.Fatalf("源集合 %s 数据不一致, _id:%v", srcColl.Name(), id.String())
	}

	// 比对后续数据
	success := int64(1)
	progres := int64(0)
	limit := int64(1)
	findOptions := options.FindOptions{
		MaxTime: &timeout,
		Sort:    bson.D{{Key: "_id", Value: 1}},
		Skip:    &stepSize,
		Limit:   &limit,
	}
	for i := int64(1); i < sampleSize; i++ {
		cur, err := srcColl.Find(context.Background(), bson.M{"_id": bson.M{"$gte": id}}, &findOptions)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				log.Printf("get out, id:%v, stepSize:%d, sampleSize:%d, i:%d", id.String(), stepSize, sampleSize, i)
				break
			}
			log.Fatalf("获取源集合 %s 第%d条数据失败: %v", srcColl.Name(), currentIndex+stepSize*i, err)
		}
		if !cur.Next(context.Background()) {
			// 源集合没有数据了
			break
		}

		id = cur.Current.Lookup("_id")
		dstDoc, err = dstColl.FindOne(context.Background(), bson.M{"_id": id}).Raw()
		if err != nil {
			if err == mongo.ErrNoDocuments && *continueNotExist {
				log.Printf("目标集合 %s 没有对应的数据, _id:%v", dstColl.Name(), id.String())
				cur.Close(context.Background())
				continue
			}
			log.Fatalf("获取目标集合 %s 对应的数据失败, _id:%v, err: %v", srcColl.Name(), id.String(), err)
		}
		if !bytes.Equal(cur.Current, dstDoc) {
			if len(cur.Current) < 200 && len(dstDoc) < 200 {
				log.Printf("源集合 %s 数据不一致, _id:%v\n源集群数据:%s\n目标集群数据:%s",
					srcColl.Name(), id.String(), cur.Current.String(), dstDoc.String())
			}
			log.Fatalf("源集合 %s 数据不一致, _id:%v", srcColl.Name(), id.String())
		}

		cur.Close(context.Background())

		success++
		if (success * 100 / sampleSize) > progres {
			progres = success * 100 / sampleSize
			log.Printf("集合 %s 抽样数据 %d 条, 进度: %d%%", srcColl.Name(), success, progres)
		}
		// log.Printf("_id:%v, 源集合 %s 第%d条数据一致", id.String(), srcColl.Name(), currentIndex+stepSize*i)
	}
	log.Printf("集合 %s 抽样数据成功 %d 条, 检查完成", srcColl.Name(), success)
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

	/*
	 * 连接集群
	 */
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

	/*
	 * 检查 db 是否存在
	 */
	if !hasDatabase(srcClient, *db) {
		log.Fatalf("源集群数据库 %s 不存在", *db)
	}
	if !hasDatabase(dstClient, *db) {
		log.Fatalf("目标集群数据库 %s 不存在", *db)
	}

	srcDB := srcClient.Database(*db)
	dstDB := dstClient.Database(*db)

	/*
	 * 指定集合进行校验
	 */
	if *coll != "" {
		if !hasCollection(srcDB, *coll) {
			log.Fatalf("源集群集合 %s 不存在", *coll)
		}
		if !hasCollection(dstDB, *coll) {
			log.Fatalf("目标集群集合 %s 不存在", *coll)
		}
		srcColl := srcDB.Collection(*coll)
		dstColl := dstDB.Collection(*coll)
		if *checkIndex {
			checkIndexes(srcColl, dstColl)
		}
		checkCollection(srcColl, dstColl)
		return
	}

	/*
	 * 对整个库进行校验
	 */
	srcColls, err := srcDB.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		log.Fatalf("源集群获取集合列表失败: %v", err)
	}
	for _, collName := range srcColls {
		if !hasCollection(dstDB, collName) {
			log.Fatalf("目标集群集合 %s 不存在", collName)
		}
		srcColl := srcDB.Collection(collName)
		dstColl := dstDB.Collection(collName)
		if *checkIndex {
			checkIndexes(srcColl, dstColl)
		}
		checkCollection(srcColl, dstColl)
	}
	log.Println("所有集合检查完成")
}
