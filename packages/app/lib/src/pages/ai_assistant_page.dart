import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../models/ai_answer.dart';

class AiAssistantPage extends StatefulWidget {
  const AiAssistantPage({
    super.key,
    required this.courseCode,
    required this.lectureNo,
    this.api,
  });

  final String courseCode;
  final int lectureNo;
  final ApiClient? api;

  @override
  State<AiAssistantPage> createState() => _AiAssistantPageState();
}

class _Turn {
  const _Turn({required this.question, required this.answer});
  final String question;
  final AiAnswer answer;
}

class _AiAssistantPageState extends State<AiAssistantPage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  final TextEditingController _input = TextEditingController();
  final List<_Turn> _turns = <_Turn>[];
  bool _sending = false;
  String? _error;
  int _remaining = -1;

  @override
  void dispose() {
    _input.dispose();
    super.dispose();
  }

  Future<void> _send() async {
    final question = _input.text.trim();
    if (question.isEmpty || _sending) return;
    setState(() {
      _sending = true;
      _error = null;
    });
    try {
      final answer = await _api.askAI(
          courseCode: widget.courseCode,
          lectureNo: widget.lectureNo,
          question: question);
      if (!mounted) return;
      _input.clear();
      setState(() {
        _turns.add(_Turn(question: question, answer: answer));
        _remaining = answer.remaining;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() => _error = e.toString().replaceFirst('Exception: ', ''));
    } finally {
      if (mounted) setState(() => _sending = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: const Text('AI 助教')),
      body: Column(
        children: <Widget>[
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(10),
            color: theme.colorScheme.secondaryContainer.withOpacity(0.35),
            child: Text(
              'AI 辅助生成，仅依据本课程资料，经教师与管理员审核的课程内容为事实源。'
              '${_remaining >= 0 ? '今日剩余 $_remaining 次。' : ''}',
              style: theme.textTheme.bodySmall,
            ),
          ),
          Expanded(
            child: ListView(
              padding: const EdgeInsets.all(12),
              children: <Widget>[
                if (_turns.isEmpty)
                  const Padding(
                    padding: EdgeInsets.all(24),
                    child: Text('可以问：这一讲的核心概念是什么？能举个例子吗？我哪里理解错了？'),
                  ),
                for (final turn in _turns) ...<Widget>[
                  Align(
                    alignment: Alignment.centerRight,
                    child: Chip(label: Text(turn.question)),
                  ),
                  Card(
                    child: Padding(
                      padding: const EdgeInsets.all(12),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: <Widget>[
                          SelectableText(turn.answer.answer),
                          if (turn.answer.citations.isNotEmpty) ...<Widget>[
                            const Divider(height: 20),
                            Text('来源',
                                style: theme.textTheme.labelMedium),
                            for (final cite in turn.answer.citations)
                              Text('第 ${cite.lectureNo} 讲 · ${cite.title}',
                                  style: theme.textTheme.bodySmall),
                          ],
                          const SizedBox(height: 6),
                          Text('AI 生成 · ${turn.answer.model}',
                              style: theme.textTheme.labelSmall),
                        ],
                      ),
                    ),
                  ),
                  const SizedBox(height: 12),
                ],
                if (_error != null)
                  Text(_error!, style: TextStyle(color: theme.colorScheme.error)),
              ],
            ),
          ),
          SafeArea(
            child: Padding(
              padding: const EdgeInsets.all(8),
              child: Row(
                children: <Widget>[
                  Expanded(
                    child: TextField(
                      controller: _input,
                      minLines: 1,
                      maxLines: 3,
                      decoration: const InputDecoration(
                        hintText: '针对本讲提问…',
                        border: OutlineInputBorder(),
                        isDense: true,
                      ),
                      onSubmitted: (_) => _send(),
                    ),
                  ),
                  const SizedBox(width: 8),
                  IconButton.filled(
                    onPressed: _sending ? null : _send,
                    icon: _sending
                        ? const SizedBox(
                            width: 18,
                            height: 18,
                            child: CircularProgressIndicator(strokeWidth: 2))
                        : const Icon(Icons.send),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}
