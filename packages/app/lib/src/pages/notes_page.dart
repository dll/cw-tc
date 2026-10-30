import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../models/note.dart';

class NotesPage extends StatefulWidget {
  const NotesPage({super.key, this.api});
  final ApiClient? api;
  @override
  State<NotesPage> createState() => _NotesPageState();
}

class _NotesPageState extends State<NotesPage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  List<Note> _items = <Note>[];
  bool _loading = true;

  @override
  void initState() { super.initState(); _load(); }

  Future<void> _load() async {
    setState(() => _loading = true);
    try {
      final items = await _api.notes();
      if (mounted) setState(() { _items = items; _loading = false; });
    } catch (_) {
      if (mounted) setState(() { _items = <Note>[]; _loading = false; });
    }
  }

  Future<void> _add() async {
    final course = TextEditingController();
    final title = TextEditingController();
    final body = TextEditingController();
    final ok = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('新建笔记'),
        content: Column(mainAxisSize: MainAxisSize.min, children: <Widget>[
          TextField(controller: course, decoration: const InputDecoration(labelText: '课程码', border: OutlineInputBorder())),
          const SizedBox(height: 8),
          TextField(controller: title, decoration: const InputDecoration(labelText: '标题（选填）', border: OutlineInputBorder())),
          const SizedBox(height: 8),
          TextField(controller: body, maxLines: 5, decoration: const InputDecoration(labelText: '笔记内容', border: OutlineInputBorder())),
        ]),
        actions: <Widget>[
          TextButton(onPressed: () => Navigator.of(dialogContext).pop(false), child: const Text('取消')),
          FilledButton(onPressed: () => Navigator.of(dialogContext).pop(true), child: const Text('保存')),
        ],
      ),
    );
    final code = course.text.trim();
    final noteTitle = title.text.trim();
    final text = body.text.trim();
    course.dispose(); title.dispose(); body.dispose();
    if (ok != true || code.isEmpty || text.isEmpty) return;
    try {
      await _api.createNote(courseCode: code, title: noteTitle, body: text);
      if (mounted) await _load();
    } catch (_) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('笔记保存失败')));
    }
  }

  Future<void> _delete(Note note) async {
    try { await _api.deleteNote(note.id); if (mounted) await _load(); } catch (_) {}
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('学习笔记')),
      floatingActionButton: FloatingActionButton(onPressed: _add, child: const Icon(Icons.add)),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _items.isEmpty
              ? const Center(child: Text('暂无笔记'))
              : RefreshIndicator(
                  onRefresh: _load,
                  child: ListView.separated(
                    padding: const EdgeInsets.all(12),
                    itemCount: _items.length,
                    separatorBuilder: (_, __) => const SizedBox(height: 8),
                    itemBuilder: (context, i) => Card(
                      child: ListTile(
                        title: Text(_items[i].title.isEmpty ? _items[i].courseCode : _items[i].title),
                        subtitle: Text(_items[i].body, maxLines: 3, overflow: TextOverflow.ellipsis),
                        trailing: IconButton(icon: const Icon(Icons.delete_outline), onPressed: () => _delete(_items[i])),
                      ),
                    ),
                  ),
                ),
    );
  }
}

