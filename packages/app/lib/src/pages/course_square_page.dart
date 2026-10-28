import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../api/api_client.dart';
import '../models/course.dart';
import '../models/major.dart';

/// 课程广场（PRD v2 §7.2 B-1）。
///
/// 接入 `GET /api/v1/courses`（公开只读，按 `status=已上架` 过滤）：
/// 支持关键词搜索 / 专业筛选 / 分页加载；加载中、失败、空态、列表如实呈现，不编造数据。
class CourseSquarePage extends StatefulWidget {
  const CourseSquarePage({super.key, this.api});

  final ApiClient? api;

  @override
  State<CourseSquarePage> createState() => _CourseSquarePageState();
}

class _CourseSquarePageState extends State<CourseSquarePage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  final TextEditingController _searchCtrl = TextEditingController();
  final ScrollController _scrollCtrl = ScrollController();

  final List<Course> _courses = <Course>[];
  List<Major> _majors = const <Major>[];
  Map<String, String> _entitlementSources = const <String, String>{};
  int? _selectedMajorId;
  String _query = '';
  int _page = 1;
  int _total = 0;
  bool _loading = false;
  bool _loadingMore = false;
  bool _failed = false;

  static const int _pageSize = 20;

  @override
  void initState() {
    super.initState();
    _scrollCtrl.addListener(_onScroll);
    _load(reset: true);
    _loadMajors();
    _loadEntitlementSources();
  }

  @override
  void dispose() {
    _searchCtrl.dispose();
    _scrollCtrl.dispose();
    super.dispose();
  }

  Future<void> _loadMajors() async {
    try {
      final majors = await _api.majors();
      if (!mounted) return;
      setState(() => _majors = majors);
    } catch (_) {
      // 专业分类加载失败不影响课程列表。
    }
  }

  Future<void> _loadEntitlementSources() async {
    try {
      final sources = await _api.myEntitlementSources();
      if (!mounted) return;
      setState(() => _entitlementSources = sources);
    } catch (_) {
      // 未登录或查询失败：按公开价展示。
    }
  }

  bool _freeGranted(String courseCode) {
    final source = _entitlementSources[courseCode];
    return source == 'grant' || source == 'free';
  }

  Future<void> _load({required bool reset}) async {
    if (_loading || _loadingMore) return;
    setState(() {
      if (reset) {
        _loading = true;
        _failed = false;
      } else {
        _loadingMore = true;
      }
    });
    final int nextPage = reset ? 1 : _page + 1;
    try {
      final result = await _api.courses(
        majorId: _selectedMajorId,
        q: _query,
        page: nextPage,
        pageSize: _pageSize,
      );
      if (!mounted) return;
      setState(() {
        if (reset) _courses.clear();
        _courses.addAll(result.items);
        _total = result.total;
        _page = result.page;
        _loading = false;
        _loadingMore = false;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _loadingMore = false;
        if (reset) _failed = true;
      });
    }
  }

  bool get _hasMore => _courses.length < _total;

  void _onScroll() {
    if (!_scrollCtrl.hasClients) return;
    final position = _scrollCtrl.position;
    if (position.pixels >= position.maxScrollExtent - 240 &&
        _hasMore &&
        !_loading &&
        !_loadingMore) {
      _load(reset: false);
    }
  }

  void _selectMajor(int? id) {
    setState(() => _selectedMajorId = id);
    _load(reset: true);
  }

  void _submitSearch(String value) {
    final next = value.trim();
    if (next == _query) return;
    setState(() => _query = next);
    _load(reset: true);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('课程广场')),
      body: Column(
        children: <Widget>[
          Padding(
            padding: const EdgeInsets.fromLTRB(12, 12, 12, 4),
            child: TextField(
              controller: _searchCtrl,
              textInputAction: TextInputAction.search,
              onSubmitted: _submitSearch,
              decoration: InputDecoration(
                hintText: '搜索课程名称或简介',
                prefixIcon: const Icon(Icons.search),
                suffixIcon: _query.isEmpty
                    ? null
                    : IconButton(
                        icon: const Icon(Icons.clear),
                        tooltip: '清除',
                        onPressed: () {
                          _searchCtrl.clear();
                          _submitSearch('');
                        },
                      ),
                isDense: true,
                border: const OutlineInputBorder(),
              ),
            ),
          ),
          if (_majors.isNotEmpty)
            _MajorFilterBar(
              majors: _majors,
              selected: _selectedMajorId,
              onSelect: _selectMajor,
            ),
          Expanded(child: _buildBody(context)),
        ],
      ),
    );
  }

  Widget _buildBody(BuildContext context) {
    if (_loading) {
      return const Center(child: CircularProgressIndicator());
    }
    if (_failed) {
      return _ErrorState(onRetry: () => _load(reset: true), message: '课程列表加载失败');
    }
    if (_courses.isEmpty) {
      return _EmptyState(
        onRetry: () => _load(reset: true),
        query: _query,
      );
    }
    return ListView.separated(
      controller: _scrollCtrl,
      padding: const EdgeInsets.all(12),
      itemCount: _courses.length + 1,
      separatorBuilder: (_, __) => const SizedBox(height: 8),
      itemBuilder: (BuildContext context, int index) {
        if (index >= _courses.length) {
          return _FooterIndicator(hasMore: _hasMore, loading: _loadingMore);
        }
        final course = _courses[index];
        return _CourseCard(
          course: course,
          freeGranted: _freeGranted(course.courseCode),
        );
      },
    );
  }
}

class _FooterIndicator extends StatelessWidget {
  const _FooterIndicator({required this.hasMore, required this.loading});

  final bool hasMore;
  final bool loading;

  @override
  Widget build(BuildContext context) {
    if (loading) {
      return const Padding(
        padding: EdgeInsets.symmetric(vertical: 16),
        child: Center(child: CircularProgressIndicator(strokeWidth: 2)),
      );
    }
    if (!hasMore) {
      return Padding(
        padding: const EdgeInsets.symmetric(vertical: 16),
        child: Center(
          child: Text('已显示全部', style: Theme.of(context).textTheme.bodySmall),
        ),
      );
    }
    return const SizedBox(height: 8);
  }
}

class _MajorFilterBar extends StatelessWidget {
  const _MajorFilterBar({
    required this.majors,
    required this.selected,
    required this.onSelect,
  });

  final List<Major> majors;
  final int? selected;
  final ValueChanged<int?> onSelect;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      height: 52,
      child: ListView(
        scrollDirection: Axis.horizontal,
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
        children: <Widget>[
          Padding(
            padding: const EdgeInsets.only(right: 8),
            child: ChoiceChip(
              label: const Text('全部'),
              selected: selected == null,
              onSelected: (_) => onSelect(null),
            ),
          ),
          for (final m in majors)
            Padding(
              padding: const EdgeInsets.only(right: 8),
              child: ChoiceChip(
                label: Text(m.name),
                selected: selected == m.id,
                onSelected: (_) => onSelect(m.id),
              ),
            ),
        ],
      ),
    );
  }
}

class _CourseCard extends StatelessWidget {
  const _CourseCard({required this.course, this.freeGranted = false});

  final Course course;
  final bool freeGranted;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final original = freeGranted ? null : course.originalPriceLabel;
    return Card(
      margin: EdgeInsets.zero,
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: () => context.push('/courses/${course.courseCode}'),
        child: Padding(
          padding: const EdgeInsets.all(14),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: <Widget>[
                    Text(course.name, style: theme.textTheme.titleMedium),
                    if (course.majorName != null) ...<Widget>[
                      const SizedBox(height: 4),
                      Text(
                        course.majorName!,
                        style: theme.textTheme.bodySmall
                            ?.copyWith(color: theme.colorScheme.primary),
                      ),
                    ],
                    if (course.intro != null && course.intro!.isNotEmpty) ...<Widget>[
                      const SizedBox(height: 4),
                      Text(
                        course.intro!,
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        style: theme.textTheme.bodySmall,
                      ),
                    ],
                    if (course.lectureSummary != null) ...<Widget>[
                      const SizedBox(height: 6),
                      Row(
                        children: <Widget>[
                          Icon(Icons.menu_book_outlined,
                              size: 14, color: theme.hintColor),
                          const SizedBox(width: 4),
                          Text(
                            course.lectureSummary!,
                            style: theme.textTheme.bodySmall
                                ?.copyWith(color: theme.hintColor),
                          ),
                        ],
                      ),
                    ],
                  ],
                ),
              ),
              const SizedBox(width: 12),
              Column(
                crossAxisAlignment: CrossAxisAlignment.end,
                children: <Widget>[
                  Text(
                    freeGranted ? '免费' : course.priceLabel,
                    style: theme.textTheme.titleMedium
                        ?.copyWith(color: theme.colorScheme.primary),
                  ),
                  if (original != null)
                    Text(
                      original,
                      style: theme.textTheme.bodySmall?.copyWith(
                        decoration: TextDecoration.lineThrough,
                        color: theme.hintColor,
                      ),
                    ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _EmptyState extends StatelessWidget {
  const _EmptyState({required this.onRetry, this.query = ''});

  final VoidCallback onRetry;
  final String query;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final searching = query.isNotEmpty;
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            Icon(Icons.inbox_outlined, size: 56, color: theme.disabledColor),
            const SizedBox(height: 12),
            Text(searching ? '没有匹配的课程' : '暂无已上架课程',
                style: theme.textTheme.titleMedium),
            const SizedBox(height: 6),
            Text(
              searching ? '换个关键词或清空搜索试试。' : '课程上架后会出现在这里。',
              style: theme.textTheme.bodySmall,
            ),
            const SizedBox(height: 12),
            TextButton.icon(
                onPressed: onRetry,
                icon: const Icon(Icons.refresh),
                label: const Text('刷新')),
          ],
        ),
      ),
    );
  }
}

class _ErrorState extends StatelessWidget {
  const _ErrorState({required this.onRetry, required this.message});

  final VoidCallback onRetry;
  final String message;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            Icon(Icons.error_outline, size: 56, color: theme.colorScheme.error),
            const SizedBox(height: 12),
            Text(message, style: theme.textTheme.titleMedium),
            const SizedBox(height: 12),
            TextButton.icon(
                onPressed: onRetry,
                icon: const Icon(Icons.refresh),
                label: const Text('重试')),
          ],
        ),
      ),
    );
  }
}
