​	该包没啥好说的，就是定义了一大堆的go运行时指标，常见的都有，在description.go中定义好了，什么堆分配了多少内存呀、gc了多少次呀、stop world时间呀等等都有

​	我们只要调用metrics.Read()方法，并传入我们想要的metrics的Name，go运行时就会帮我们填充该metrics对应的值

