# CHANGELOG

## 0.1.21 - (20230308)

### Feat

- 任务调度器新增`SetLogger`方法;

## 0.1.20 - (20230308)

### Feat

- 新增任务调度器`cronjob`;

## 0.1.19 - (20230227)

### Refactor

- 修改`logger.DefaultLogger`实现;

## 0.1.18 - (20230221)

### Feat

- 支持基于`iptables`的内核层的最大连接数量限制;

### Fix

- 修复`Remote.Content()`多一个字节的错误;

## 0.1.17 - (20230216)

### Feat

- 新增`Remote.Index()`api;

## 0.1.16 - (20230211)

### Fix

- 修改Drain后消息头偏移量的错误;

### Refactor

- 设置缺省状态下TcpClient的默认重连间隔为1秒;

## 0.1.15 - (20230211)

### Refactor

- 完成重写`tcp server` and `tcp client`, 提供更好的接口和性能;

## 0.1.14 - (20230203)

### Refactor

- 重写`tcp server` and `tcp client`;

## 0.1.13 - (20230202)

### Rename

- `zaplog.AllIface` to `zaplog.AIface`;

## 0.1.12 - (20230108)

### Fix

- `httpc.Httpr`未导入`req`的错误;

## 0.1.11 - (20230106)

### Feat

- 新增查找泛型方法;

## 0.1.10 - (20221229)

### Refactor

- 修改`zaplog.ConsoleLogger`的`xxxf`接口内部实现为`fmt.Errorf`;
- 新增日志接口`FIface`, `AllIface`;
- 修改`httpr`;

## 0.1.9 - (20221205)

### Refactor

- 重写`TCPs`，暂未测试`FasterServer`;

## 0.1.8 - (20221125)

### Feat

- 新增泛型类型约束;
- 修改优化TCPs;

## 0.1.7 - (20221124)

### Refactor

- `python.Repr()`优化;
- `tcps`优化;

## 0.1.6 - (20221025)

### Fix

- 修复`python.Repr`类型推断的错误，影响`zaplog.ConsoleLogger.FXX()`方法；
- 修复`environ.GetString/environ.GetInt`方法的错误；

## 0.1.5 - (20221021)

### Refactor

- 拆分`client`为若干独立的包，以最小化依赖；

## 0.1.4 - (20221012)

### Feat

- 新增`FasterJson`选项以提供`jsoniter.ConfigFastest`配置；
- 新增`json`序列化的默认选项`DefaultJson`，并允许修改；

## 0.1.3 - (20221012)

### Refactor

- 优化`python.Any`方法；
- 重写`cprint`系列方法, 移除fmt包,直接输出到控制台标准输出；
- 格式化代码;

### Feat

- 新增软件包`zaplog`;
    - 支持创建多日志句柄;
    - 支持查询已创建的日志句柄;
    - 提供默认日志句柄;

## 0.1.2 - (20221009)

### Feat

- 新增软件包`python`;

### Refactor

- 修改文件目录结构；

## 0.1.1 - (20220929)

### Refactor

- 修改文件目录结构；

## 0.1.0 - (20220929)

- 提取`flaskgo`代码库；