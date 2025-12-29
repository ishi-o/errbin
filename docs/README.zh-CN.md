# Errbin

声明式 Gin 错误处理中间件，支持层次化错误匹配和中间件链。

[English](docs/README.md) | 中文

## 特性

- **层次化错误匹配**：处理错误及其所有派生类型
- **中间件支持**：为横切关注点链接中间件
- **声明式 API**：用于定义错误处理程序的清晰、类型安全的 API
- **错误继承**：自动匹配派生错误
- **零依赖**：仅依赖 Gin 框架
- **简单集成**：Gin 应用的即插即用中间件

## 安装

```bash
go get github.com/ishi-o/errbin
```

## 快速开始

Errbin 允许您以声明式的方式为特定错误类型及其子类型定义处理程序。

### 基本用法

1. 在应用初始化期间注册错误处理程序

   ```go
   import (
   	"github.com/gin-gonic/gin"
   	"github.com/ishi-o/errbin"
   )

   func init() {
   	errbin.Use(func(err error, c *gin.Context) {
   		c.JSON(404, gin.H{"error": "资源未找到"})
   	}, ErrNotFound)

   	errbin.Use(func(err error, c *gin.Context) {
   		c.JSON(401, gin.H{"error": "未经授权"})
   	}, ErrUnauthorized)

   	r.Use(errbin.ErrbinMiddleware())
   }
   ```

2. 将 Errbin 中间件添加到您的 Gin 路由器

   ```go
   import (
   	"log"

   	"github.com/gin-gonic/gin"
   	"github.com/ishi-o/errbin"
   )

   func init() {
   	errbin.UseGlobal(func(next errbin.ErrorHandler) errbin.ErrorHandler {
   		return func(err error, c *gin.Context) {
   			log.Printf("错误: %v", err)
   			next(err, c)
   		}
   	})

   	errbin.Use(func(err error, c *gin.Context) {
   		c.JSON(500, gin.H{"error": "数据库错误"})
   	}, ErrDatabase)

   	r.Use(errbin.ErrbinMiddleware())
   }
   ```

3. 让 Errbin 自动匹配并处理错误

### 核心概念

- **错误层次结构**：处理程序使用 Go 的 `errors.Is()` 语义匹配错误
- **错误处理程序**：处理错误并生成 HTTP 响应的函数
- **错误中间件**：添加日志记录、指标等功能的包装器
- **全局中间件**：应用于所有错误处理程序的中间件
- **回退处理程序**：未处理错误的默认处理程序

## API 概览

### 核心函数

- `Use()` - 为特定错误类型注册错误处理程序
- `UseGlobal()` - 为所有处理程序注册全局中间件
- `ErrbinMiddleware()` - 处理错误的 Gin 中间件
- `Fallback()` - 为未处理错误设置自定义回退处理程序

### 中间件工具

- `MiddlewareChain()` - 将多个中间件链接在一起
- `UseWithMiddleware()` - 使用中间件注册处理程序
- `Chain()` - 链接多个处理程序（按顺序执行）

## 重要说明

- **仅在初始化时使用**：`Use()` 必须在应用程序初始化期间调用，而不是在服务器启动后
- **非并发安全**：错误注册不是线程安全的，应在服务器启动前完成
- **错误继承**：父错误的处理程序也将处理所有子错误
- **回退处理程序**：始终为意外错误设置有意义的回退处理程序

## 许可证

MIT 许可证 - 详见 LICENSE 文件

## 致谢

基于 [Gin Web 框架](https://github.com/gin-gonic/gin) 构建，受层次化错误处理模式启发。
