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
    this.contentReviewing = false,
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

  /// 课程存在待处理的紧急纠错申请：学习不受影响，但暂停新测评/发证。
  final bool contentReviewing;

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
      contentReviewing: json['content_reviewing'] == true,
    );
  }

  /// 人类可读的大小。
  String get sizeLabel {
    if (sizeBytes <= 0) return '';
    if (sizeBytes < 1024) return '$sizeBytes B';
    if (sizeBytes < 1024 * 1024) {
      return '${(sizeBytes / 1024).toStringAsFixed(1)} KB';
    }
    return '${(sizeBytes / (1024 * 1024)).toStringAsFixed(1)} MB';
  }

  /// 文件名（storage_key 的末段，如 `README.md`）。
  String get fileName {
    final idx = storageKey.lastIndexOf('/');
    return idx >= 0 ? storageKey.substring(idx + 1) : storageKey;
  }

  /// 文本类资源（讲义 Markdown / 代码 / HTML / 纯文本 / 冻结输出）可端内阅读。
  bool get isTextKind =>
      kind == 'markdown' ||
      kind == 'code' ||
      kind == 'html' ||
      kind == 'text' ||
      kind == 'frozen_output' ||
      _textExts.contains(_ext);

  /// 可端内预览的图片资源（通常来自 HTML 讲义内联资源或独立图片资产）。
  bool get isImageKind => kind == 'image' || _imageExts.contains(_ext);

  /// 可端内预览的 PDF（以扩展名识别；kind 可能尚未单独建模）。
  bool get isPdfKind => _ext == '.pdf' || kind == 'pdf';

  /// 可通过浏览器/播放器 URL 播放的媒体（视频/音频且已拿到直链或签名媒体地址）。
  bool get isPlayableMedia =>
      (kind == 'video' || kind == 'audio') && (url != null && url!.isNotEmpty);

  String get _ext {
    final lower = fileName.toLowerCase();
    final idx = lower.lastIndexOf('.');
    return idx >= 0 ? lower.substring(idx) : '';
  }

  static const Set<String> _imageExts = <String>{
    '.png',
    '.jpg',
    '.jpeg',
    '.gif',
    '.webp',
    '.bmp',
    '.svg',
  };

  static const Set<String> _textExts = <String>{
    '.txt',
    '.md',
    '.markdown',
    '.json',
    '.yaml',
    '.yml',
    '.xml',
    '.csv',
    '.log',
    '.ini',
    '.conf',
    '.properties',
    '.c',
    '.h',
    '.hpp',
    '.hh',
    '.cpp',
    '.cc',
    '.cxx',
    '.java',
    '.kt',
    '.py',
    '.js',
    '.ts',
    '.tsx',
    '.jsx',
    '.vue',
    '.go',
    '.rs',
    '.cs',
    '.swift',
    '.sql',
    '.sh',
    '.bat',
    '.ps1',
    '.rb',
    '.php',
    '.dart',
  };
}
