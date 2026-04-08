#!/bin/bash
# 验证完整 Dashboard 的所有指标

export KUBECONFIG="/Users/xiaodian/.kube/huawei-pref-kubeconfig.yaml"

echo "=================================================="
echo "验证完整 Dashboard 的所有指标"
echo "=================================================="
echo ""

# 函数：测试指标是否有数据
test_metric() {
  local metric=$1
  local desc=$2

  # 查询指标
  result=$(curl -s "http://localhost:15020/stats/prometheus" 2>/dev/null | grep "^${metric}{" | wc -l)

  if [ "$result" -gt 0 ]; then
    echo "✅ $desc"
    echo "   指标: $metric"
    echo "   数据点: $result"
    # 显示第一个值
    first_value=$(curl -s "http://localhost:15020/stats/prometheus" 2>/dev/null | grep "^${metric}{" | head -1)
    echo "   示例: $first_value"
    echo ""
  else
    echo "❌ $desc"
    echo "   指标: $metric"
    echo "   错误: 没有数据"
    echo ""
  fi
}

echo "=================================================="
echo "1. Go 进程内存总览"
echo "=================================================="
test_metric "istio_agent_go_memstats_sys_bytes" "Go 进程总内存 (Sys)"
test_metric "istio_agent_process_resident_memory_bytes" "Go 进程常驻内存 (RSS)"
test_metric "istio_agent_process_virtual_memory_bytes" "Go 进程虚拟内存 (VMS)"

echo "=================================================="
echo "2. Go 内存堆 (Heap) 情况"
echo "=================================================="
test_metric "istio_agent_go_memstats_heap_inuse_bytes" "Heap 使用中"
test_metric "istio_agent_go_memstats_heap_idle_bytes" "Heap 空闲/缓存"
test_metric "istio_agent_go_memstats_heap_sys_bytes" "Heap 系统分配"

echo "=================================================="
echo "3. Go 内存栈 (Stack) 详情"
echo "=================================================="
test_metric "istio_agent_go_memstats_stack_inuse_bytes" "Stack 使用中"
test_metric "istio_agent_go_memstats_stack_sys_bytes" "Stack 系统分配"
test_metric "istio_agent_go_memstats_mspan_inuse_bytes" "MSpan 使用中"
test_metric "istio_agent_go_memstats_mcache_inuse_bytes" "MCache 使用中"

echo "=================================================="
echo "4. Go 系统开销"
echo "=================================================="
test_metric "istio_agent_go_memstats_other_sys_bytes" "其他系统内存"
test_metric "istio_agent_go_memstats_gc_sys_bytes" "GC 系统内存"
test_metric "istio_agent_go_memstats_buck_hash_sys_bytes" "Bucket Hash 内存"

echo "=================================================="
echo "5. Envoy 内存情况"
echo "=================================================="
test_metric "envoy_server_memory_allocated" "Envoy 已分配内存"
test_metric "envoy_server_memory_heap_size" "Envoy 堆大小"
test_metric "envoy_server_memory_physical_size" "Envoy 物理内存"

echo "=================================================="
echo "6. 其他指标"
echo "=================================================="
test_metric "istio_agent_go_goroutines" "Goroutines 数量"
test_metric "istio_agent_go_threads" "Threads 数量"
test_metric "istio_agent_go_memstats_alloc_bytes_total" "内存分配总计"
test_metric "istio_agent_go_memstats_heap_objects" "堆对象数量"

echo "=================================================="
echo "7. GC 统计"
echo "=================================================="
test_metric "istio_agent_go_gc_duration_seconds_sum" "GC 耗时总计"
test_metric "istio_agent_go_gc_duration_seconds_count" "GC 次数总计"

echo "=================================================="
echo "8. 进程资源"
echo "=================================================="
test_metric "istio_agent_process_open_fds" "打开的文件描述符"
test_metric "istio_agent_process_max_fds" "最大文件描述符"

echo "=================================================="
echo "9. Pod 实际内存"
echo "=================================================="
echo "✅ Pod 实际内存 (kubectl top)"
kubectl top pod -n agent higress-allinone-5cc7d4c7d-kgnk9
echo ""

echo "=================================================="
echo "验证完成！"
echo "=================================================="
echo ""
echo "下一步: 将 higress-complete-dashboard.json 导入 Grafana"
echo "所有指标都应该显示数据！"
