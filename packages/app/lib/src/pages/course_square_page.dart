import 'package:flutter/material.dart';

/// 课程广场（PRD v2 §7.2 B-1）。
///
/// M0 为骨架：搭出“搜索/筛选栏 + 列表”的信息架构与空态。
/// 诚实原则（AGENTS.md 铁律）：**未接入数据源前不编造课程、不显示“共 N 门课”**，
/// 因此列表区为明确的“待接入”空态，而非假数据。
/// M1 起：课程来自上游 `课程总览.json`（只读同步），专业分类/价格/上架状态
/// 来自平台自有元数据（PRD v2 §8.4 边界表），按 `status=已上架` 过滤。
class CourseSquarePage extends StatelessWidget {
  const CourseSquarePage({super.key});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: const Text('课程广场')),
      body: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          // 搜索 / 筛选栏（M1 接入数据后启用）
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 12, 16, 8),
            child: Row(
              children: <Widget>[
                Expanded(
                  child: TextField(
                    enabled: false,
                    decoration: InputDecoration(
                      isDense: true,
                      prefixIcon: const Icon(Icons.search, size: 20),
                      hintText: '搜索课程（M1 开放）',
                      border: OutlineInputBorder(borderRadius: BorderRadius.circular(10)),
                    ),
                  ),
                ),
                const SizedBox(width: 8),
                IconButton.filledTonal(onPressed: null, icon: const Icon(Icons.tune), tooltip: '筛选（M1 开放）'),
              ],
            ),
          ),
          SizedBox(
            height: 40,
            child: ListView(
              scrollDirection: Axis.horizontal,
              padding: const EdgeInsets.symmetric(horizontal: 12),
              children: const <Widget>[
                Padding(padding: EdgeInsets.only(right: 8), child: ChoiceChip(label: Text('全部'), selected: true, onSelected: null)),
                Padding(padding: EdgeInsets.only(right: 8), child: ChoiceChip(label: Text('专业分类'), selected: false, onSelected: null)),
                Padding(padding: EdgeInsets.only(right: 8), child: ChoiceChip(label: Text('价格'), selected: false, onSelected: null)),
              ],
            ),
          ),
          const Divider(height: 1),
          Expanded(
            child: Center(
              child: Padding(
                padding: const EdgeInsets.all(24),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: <Widget>[
                    Icon(Icons.inbox_outlined, size: 56, color: theme.disabledColor),
                    const SizedBox(height: 12),
                    Text('课程列表尚未接入数据源', style: theme.textTheme.titleMedium),
                    const SizedBox(height: 6),
                    Text('M1 起接入课程数据后，这里展示“按专业筛选 / 主线辅线 / 价格”。',
                        textAlign: TextAlign.center, style: theme.textTheme.bodySmall),
                  ],
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}
