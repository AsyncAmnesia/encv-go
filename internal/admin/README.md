文档：https://goframe.org.cn/docs/

假设 gf 可执行文件已经存在项目根路径


```cmd
./gf gen ctrl -s "pkg/admin/api" -d "internal/admin/controller"
```

该命令通过分析给定的 `api` 接口定义目录下的代码，自动生成对应的控制器/ `SDK Go` 代码文件。
