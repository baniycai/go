#!/bin/bash

# 遍历所有Go文件
for file in $(find . -type f -name '*.go'); do
    # 将每个Go文件中的import路径添加std/前缀
    sed -i 's/import "/import "std\//g' "$file"
done
