import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../models/note.dart';

class SuggestionsPage extends StatefulWidget {
  const SuggestionsPage({super.key, this.api});
  final ApiClient? api;
  @override
  State<SuggestionsPage> createState() => _SuggestionsPageState();
}

class _SuggestionsPageState extends State<SuggestionsPage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  List<Suggestion> _items = <Suggestion>[];
  bool _loading = true;

  @override
  void initState() { super.initState(); _load(); }

  Future<void> _load() async {
    setState(() => _loading = true);
    try {
      final items = await _api.suggestions();
      if (mounted) setState(() { _items = items; _loading = false; });
    } catch (_) {
      if (mounted) setState(() { _items = <Suggestion>[]; _loading = false; });
    }
  }

  Future<void> _add() async {
    final body = TextEditingController();
    final ok = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('提交建议'),
        content: TextField(controller: body, maxLines: 5, decoration: const InputDecoration(labelText: '建议内容（10-1000 字）', border: OutlineInputBorder())),
        actions: <Widget>[
          TextButton(onPressed: () => Navigator.of(dialogContext).pop(false), child: const Text('取消')),
          FilledButton(onPressed: () => Navigator.of(dialogContext).pop(true), child: const Text('提交')),
        ],
      ),
    );
    final text = body.text.trim();
    body.dispose();
    if (ok != true || text.length < 10) return;
    try {
      await _api.createSuggestion(text);
      if (mounted) await _load();
    } catch (_) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('提交失败')));
    }
  }

  String _status(String value) {
    switch (value) {
      case 'accepted': return '已采纳';
      case 'rejected': return '未采纳';
      case 'planned': return '已排期';
      default: return '处理中';
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('我的建议')),
      floatingActionButton: FloatingActionButton(onPressed: _add, child: const Icon(Icons.add_comment_outlined)),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _items.isEmpty
              ? const Center(child: Text('暂无建议'))
              : RefreshIndicator(
                  onRefresh: _load,
                  child: ListView.separated(
                    padding: const EdgeInsets.all(12),
                    itemCount: _items.length,
                    separatorBuilder: (_, __) => const SizedBox(height: 8),
                    itemBuilder: (context, i) => Card(
                      child: ListTile(
                        title: Text(_items[i].body, maxLines: 3, overflow: TextOverflow.ellipsis),
                        subtitle: Text(_status(_items[i].status) + (_items[i].reply == null ? '' : ' · ${_items[i].reply}')),
                      ),
                    ),
                  ),
                ),
    );
  }
}
