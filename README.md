# functools

- 一些可以复用的代码库，提供稳定的接口；
- 按需导入使用，也可以复制相关代码后使用；

### `struct`内存对齐

```bash
go install golang.org/x/tools/go/analysis/passes/fieldalignment/cmd/fieldalignment@latest

fieldalignment -fix ./... 
```
