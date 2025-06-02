# mongocheck
对 2 个 MongoDB 集群的数据进行抽样调查。
> 工具假设：一个表的 _id 类型是一样的，比如都是 ObjectId（默认）或者都是数值类型等。

# 编译
安装 go 环境，推荐 1.22 及以上版本
sh build.sh

如果源集群的版本低于 3.6 ，请使用 3.4 分支的代码编译。

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

# 不同采样算法的对比

## 理论对比
### SkipLimit 算法
优点1：根据 _id 升序采样，由于默认的 _id 具有时间属性，因此采样的数据从旧到新。     
优点2：采样的步长比较固定，采样分布比较均匀。    

缺点：底层的执行计划是 indexScan，性能比较差。

在分片集群场景下，需要从 shard->mongos 传输大量的数据，然后在 mongos 上执行 skipLimit 算法，因此存在性能下降问题，具体和 skip 大小和分片数量有关。

### Sample 算法
优点1：足够随机性，而且在采样数据量较少（<= 5%）的场景下，randomCursor 的性能很好。    
缺点1：采样数据量超 5% 时，会进入 top-k 排序阶段，性能下降明显。    
缺点2：采样数据量超 5% 时，数据总量超过 100M 时，进入外部排序， 空间膨胀明显（在 _tmp 目录下会有很多临时文件）。    

在分片集群场景下，存在一定的请求放大。     

### SampleRate/rand 算法
优点1： CollScan 全表扫，性能稳定。       
缺点1： CollScan 在小数据量时性能差，因为有很多不必要的扫描。     
缺点2： 由于随机数的特性，真实采样的数据量可能存在偏差。比如预期采样 100 条，实际可能采样到 95 或者 102 条等。    

## 性能对比
### 副本集
硬件环境：2C4G 云主机    
部署模式：2 个 mongod 单实例，内核版本是 5.0    
数据集：使用 YCSB 导入 1,000,000 条数据，每条数据 1KB    

| 表头1 | 表头2 | 表头3 |
|-------|-------|-------|
| 单元格1 | 单元格2 | 单元格3 |
| 单元格4 | 单元格5 | 单元格6 |


| 采样模式	      |0.10%	|1%	|5%	|10%	|20%	|50%	|80%	|100%|
|:--|:--|:--|:--|:--|:--|:--|:--|:--|    
|skip	            |2	|21	|69	|131	|236	|570	|995	|1257|    
|sample		|1     |10	|36	|69	|99	|222	|322	|403|    
|sampleRate/random	|6	|10	|27	|40	|60	|140	|217	|268|    

### 分片集群
硬件环境：2C8G 云主机    
部署模式：一个 2 Shard 的分片集群作为源集群，一个单实例的 mongod 作为目标集群，内核版本是 8.0    
数据集：使用 YCSB 导入 1,000,000 条数据，每条数据 1KB     

采样模式          0.10%	1%	5%	10%	20%	50%	80%	100%
skip	            22	28	67	111	205	491	759	950
sample	      1	7	8	12	23	49	75	95
sampleRate/random	1	2	4	9	16	39	60	75

