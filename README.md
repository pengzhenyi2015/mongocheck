# mongocheck
对 2 个 MongoDB 集群的数据进行抽样调查。
> 工具假设：一个表的 _id 类型是一样的，比如都是 ObjectId（默认）或者都是数值类型等。

# 编译
安装 go 环境，推荐 1.22 及以上版本
sh build.sh

# 用法
```
Usage of ./mongocheck:
  -checkIndex
        是否比对索引
  -coll string
        要检查的集合名, 可选, 如果不指定则检查所有集合
  -continueNotExist
        目标集群有数据不存在时,是否报错继续检查。一般目标集群一直处于增量同步的情况下考虑使用
  -count int
        每个表要抽样检查的数据条数 (default 100)
  -db string
        要检查的数据库名, 必填
  -dst string
        源集群地址, 必填 (default "mongodb://testUser:testPwd@localhost:27018/admin?authSource=admin")
  -mode string
        要检查的模型名, 可选 skip|sample|sampleRate(源集群5.0及以上版本)|rand(源集群5.0及以上版本)
        使用 sample 模式需要小心，如果 sample 的数据条数超过总数的 5%，会进入 top-k 排序，可能会涉及到外部排序
        参考 https://www.mongodb.com/docs/manual/reference/operator/aggregation/sample/
        如果使用 sampleRate 和 rand 模式, 由于随机数的原因, 实际抽样的数据条数和指定的数据条数可能存在一定的误差 (default "skip")
  -rate float
        每个表要抽样检查的比例，取值为 0到1 的小数。如果同时指定了count,则取两者的最小值 (default 0.01)
  -src string
        源集群地址, 必填 (default "mongodb://testUser:testPwd@localhost:27017/admin?authSource=admin")
```

# 示例
## 1. 校验整个db
```
./mongocheck -src='mongodb://testuser:testpwd@localhost:27017/admin?authSource=admin&readPreference=secondaryPreferred' -dst='mongodb://testuser:testpwd@localhost:27018/admin?authSource=admin&readPreference=secondaryPreferred'  -db=db1 -count=9999 -rate=0.1
```

## 2. 检验单个collection
```
./mongocheck -src='mongodb://testuser:testpwd@localhost:27017/admin?authSource=admin&readPreference=secondaryPreferred' -dst='mongodb://testuser:testpwd@localhost:27018/admin?authSource=admin&readPreference=secondaryPreferred'  -db=db1  -coll=coll2 -count=9999 -rate=0.1
```
