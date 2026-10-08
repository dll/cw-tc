import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../models/exam.dart';

/// 测评页（PRD v2 §7.7 C）：开考 → 答题 → 交卷 → 判分/发证。
///
/// 试卷不含答案；单题即时选择；交卷后由服务端判分（客户端不判）。
class ExamPage extends StatefulWidget {
  const ExamPage({super.key, required this.courseCode, required this.level, this.api});

  final String courseCode;
  final String level;
  final ApiClient? api;

  @override
  State<ExamPage> createState() => _ExamPageState();
}

class _ExamPageState extends State<ExamPage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  Future<ExamSession>? _sessionFuture;
  ExamSession? _session;
  final Map<String, String> _answers = <String, String>{};
  bool _submitting = false;
  ExamResult? _result;
  String? _error;

  @override
  void initState() {
    super.initState();
    _sessionFuture = _api.startExam(courseCode: widget.courseCode, level: widget.level);
    _sessionFuture!.then((s) {
      if (mounted) setState(() => _session = s);
    }).catchError((_) {
      if (mounted) setState(() => _error = '开考失败（可能无权益或冷却中）');
    });
  }

  Future<void> _submit() async {
    if (_session == null || _submitting) return;
    setState(() => _submitting = true);
    try {
      final result = await _api.submitExam(_session!.sessionId, _answers);
      if (mounted) setState(() => _result = result);
    } catch (_) {
      if (mounted) setState(() => _error = '交卷失败');
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: Text('测评 · ${widget.level}')),
      body: _buildBody(theme),
      bottomNavigationBar: (_session != null && _result == null)
          ? SafeArea(
              child: Padding(
                padding: const EdgeInsets.all(12),
                child: FilledButton(
                  onPressed: _submitting ? null : _submit,
                  child: Text(_submitting ? '判分中…' : '交卷'),
                ),
              ),
            )
          : null,
    );
  }

  Widget _buildBody(ThemeData theme) {
    if (_result != null) return _ResultView(result: _result!);
    if (_error != null) return Center(child: Text(_error!, style: TextStyle(color: theme.colorScheme.error)));
    if (_session == null) return const Center(child: CircularProgressIndicator());
    final paper = _session!.paper;
    return ListView.separated(
      padding: const EdgeInsets.all(16),
      itemCount: paper.length,
      separatorBuilder: (_, __) => const Divider(height: 24),
      itemBuilder: (BuildContext context, int i) {
        final q = paper[i];
        return _QuestionView(
          index: i,
          question: q,
          selected: _answers[q.qno],
          onChanged: (v) => setState(() => _answers[q.qno] = v),
        );
      },
    );
  }
}

class _QuestionView extends StatelessWidget {
  const _QuestionView({required this.index, required this.question, required this.selected, required this.onChanged});

  final int index;
  final ExamQuestion question;
  final String? selected;
  final ValueChanged<String> onChanged;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final options = question.options;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        Text('${index + 1}. ${question.stem}', style: theme.textTheme.titleMedium),
        const SizedBox(height: 4),
        Text('${question.score} 分', style: theme.textTheme.bodySmall),
        const SizedBox(height: 12),
        if (options != null && options.isNotEmpty)
          ...options.map((o) => RadioListTile<String>(
                dense: true,
                title: Text(o),
                value: o,
                groupValue: selected,
                onChanged: (v) => onChanged(v!),
              ))
        else if (question.type == 'judge')
          Row(
            children: <Widget>[
              ChoiceChip(label: const Text('对'), selected: selected == '对', onSelected: (_) => onChanged('对')),
              const SizedBox(width: 8),
              ChoiceChip(label: const Text('错'), selected: selected == '错', onSelected: (_) => onChanged('错')),
            ],
          )
        else
          TextField(
            decoration: const InputDecoration(labelText: '作答', isDense: true, border: OutlineInputBorder()),
            onChanged: onChanged,
          ),
      ],
    );
  }
}

class _ResultView extends StatelessWidget {
  const _ResultView({required this.result});

  final ExamResult result;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            Icon(
              result.isPass ? Icons.emoji_events : Icons.sentiment_dissatisfied,
              size: 64,
              color: result.isPass ? theme.colorScheme.primary : theme.disabledColor,
            ),
            const SizedBox(height: 12),
            Text(result.isPass ? '通过' : '未通过', style: theme.textTheme.headlineSmall),
            const SizedBox(height: 8),
            Text('总分 ${result.totalScore}（自动判 ${result.autoScore}）', style: theme.textTheme.titleMedium),
            if (result.certNo != null && result.certNo!.isNotEmpty) ...<Widget>[
              const SizedBox(height: 12),
              Text('已发证：${result.certNo}', style: theme.textTheme.bodyMedium),
            ],
          ],
        ),
      ),
    );
  }
}
