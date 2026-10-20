/// 讲次资源（对应 `GET /courses/:code/lectures/:no/resources` 返回项）。
class LectureResource {
  const LectureResource({
    required this.kind,
    required this.title,
    required this.storageKey,
    required this.sizeBytes,
    required this.expiresAt,
    required this.signature,
    this.url,
    this.inlineBaseUrl,
  });

  final String kind; // 讲义 / 课件 / 视频 等
  final String title;
  final String storageKey;
  final int sizeBytes;
  final int expiresAt; // unix 秒
  final String signature;

  /// 视频/音频的 COS 预签名直链（自认证，Web 播放器无需带 Bearer 头）。其余资源为 null。
  final String? url;

  /// HTML 内相对音频等资源使用的短时授权基址。
  final String? inlineBaseUrl;

  factory LectureResource.fromJson(Map<String, dynamic> json) {
    return LectureResource(
      kind: json['kind'] as String,
      title: json['title'] as String,
      storageKey: json['storage_key'] as String,
      sizeBytes: (json['size_bytes'] as num?)?.toInt() ?? 0,
      expiresAt: (json['expires_at'] as num?)?.toInt() ?? 0,
      signature: json['signature'] as String,
      url: json['url'] as String?,
      inlineBaseUrl: json['inline_base_url'] as String?,
    );
  }

  /// 人类可读的大小。
  String get sizeLabel {
    if (sizeBytes <= 0) return '';
    if (sizeBytes < 1024) return '$sizeBytes B';
    if (sizeBytes < 1024 * 1024)
      return '${(sizeBytes / 1024).toStringAsFixed(1)} KB';
    return '${(sizeBytes / (1024 * 1024)).toStringAsFixed(1)} MB';
  }

  /// 文件名（storage_key 的末段，如 `README.md`）。
  String get fileName {
    final idx = storageKey.lastIndexOf('/');
    return idx >= 0 ? storageKey.substring(idx + 1) : storageKey;
  }

  /// 文本类资源（讲义 Markdown / 代码 / HTML）可端内阅读。
  bool get isTextKind => kind == 'markdown' || kind == 'code' || kind == 'html';
}
