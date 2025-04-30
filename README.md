# mongocheck
对 2 个 MongoDB 集群的数据进行抽样调查

# 编译
安装 go 环境，推荐 1.22 及以上版本
sh build.sh

# 用法
```
Usage of ./mongocheck:
  -coll string
        要检查的集合名, 可选, 如果不指定则检查所有集合
  -count int
        每个表要抽样检查的数据条数 (default 100)
  -db string
        要检查的数据库名, 必填
  -dst string
        源集群地址, 必填 (default "mongodb://testUser:testPwd@localhost:27018/admin?authSource=admin")
  -percent int
        每个表要抽样检查的数据百分比,如果同时指定了count,则取两者的最小值 (default 10)
  -src string
        源集群地址, 必填 (default "mongodb://testUser:testPwd@localhost:27017/admin?authSource=admin")
```

# 示例
## 1. 校验整个db
```
./mongocheck -src=mongodb://testuser:testpwd@localhost:27017/admin?authSource=admin -dst=mongodb://testuser:testpwd@localhost:27018/admin?authSource=admin  -db=db1 -count=9999 -percent=10
```

## 2. 检验单个collection
```
./mongocheck -src=mongodb://testuser:testpwd@localhost:27017/admin?authSource=admin -dst=mongodb://testuser:testpwd@localhost:27018/admin?authSource=admin  -db=db1  -coll=coll2 -count=9999 -percent=10
```