/// 站内消息（`GET /me/messages`）。
class AppMessage {
  const AppMessage({
    required this.id,
    required this.type,
    required this.title,
    required this.body,
    required this.link,
    required this.read,
    required this.createdAt,
  });

  final int id;
  final String type; // system / order / exam / cert / course / broadcast
  final String title;
  final String body;
  final String link;
  final bool read;
  final String createdAt;

  String get category {
    switch (type) {
      case 'system':
        return 'system';
      case 'broadcast':
        return 'broadcast';
      default:
        return 'transaction';
    }
  }

  String get categoryLabel {
    switch (category) {
      case 'system':
        return '系统';
      case 'broadcast':
        return '广播';
      default:
        return '事务';
    }
  }

  factory AppMessage.fromJson(Map<String, dynamic> json) => AppMessage(
        id: (json['id'] as num?)?.toInt() ?? 0,
        type: json['type'] as String? ?? '',
        title: json['title'] as String? ?? '',
        body: json['body'] as String? ?? '',
        link: json['link'] as String? ?? '',
        read: json['read'] as bool? ?? false,
        createdAt: json['created_at'] as String? ?? '',
      );
}

class MessagePrefs {
  const MessagePrefs({required this.broadcastOptOut});

  final bool broadcastOptOut;

  factory MessagePrefs.fromJson(Map<String, dynamic> json) => MessagePrefs(
        broadcastOptOut: json['broadcast_opt_out'] as bool? ?? false,
      );
}

class MessagePage {
  const MessagePage({
    required this.items,
    required this.unread,
    required this.prefs,
  });

  final List<AppMessage> items;
  final int unread;
  final MessagePrefs prefs;
}
