class ImmichAsset {
  ImmichAsset({
    required this.id,
    required this.type,
    required this.originalFileName,
    required this.fileCreatedAt,
    required this.isFavorite,
    this.duration,
    this.exifInfo,
    this.thumbhash,
    this.originalMimeType,
  });

  final String id;
  // 'IMAGE' | 'VIDEO' (others normalized to IMAGE)
  final String type;
  final String originalFileName;
  final String fileCreatedAt;
  bool isFavorite;
  final String? duration;
  final ExifInfo? exifInfo;
  final String? thumbhash;
  final String? originalMimeType;

  bool get isVideo => type == 'VIDEO';
  bool get isImage => !isVideo;

  factory ImmichAsset.fromJson(Map<String, dynamic> j) {
    final rawType = (j['type'] as String? ?? 'IMAGE').toUpperCase();
    final type = rawType == 'VIDEO' ? 'VIDEO' : 'IMAGE';
    return ImmichAsset(
      id: j['id'] as String,
      type: type,
      originalFileName: j['originalFileName'] as String? ?? '',
      fileCreatedAt: j['fileCreatedAt'] as String? ?? '',
      isFavorite: j['isFavorite'] as bool? ?? false,
      duration: j['duration']?.toString(),
      thumbhash: j['thumbhash'] as String?,
      originalMimeType: j['originalMimeType'] as String?,
      exifInfo: j['exifInfo'] is Map
          ? ExifInfo.fromJson(j['exifInfo'] as Map<String, dynamic>)
          : null,
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'type': type,
        'originalFileName': originalFileName,
        'fileCreatedAt': fileCreatedAt,
        'isFavorite': isFavorite,
        'duration': duration,
        'exifInfo': exifInfo?.toJson(),
      };
}

class ExifInfo {
  ExifInfo({this.city, this.country, this.make, this.model, this.exifImageWidth, this.exifImageHeight, this.dateTimeOriginal, this.description, this.fileSizeInByte});
  final String? city;
  final String? country;
  final String? make;
  final String? model;
  final int? exifImageWidth;
  final int? exifImageHeight;
  final String? dateTimeOriginal;
  final String? description;
  final int? fileSizeInByte;

  factory ExifInfo.fromJson(Map<String, dynamic> j) => ExifInfo(
        city: j['city'] as String?,
        country: j['country'] as String?,
        make: j['make'] as String?,
        model: j['model'] as String?,
        exifImageWidth: j['exifImageWidth'] as int?,
        exifImageHeight: j['exifImageHeight'] as int?,
        dateTimeOriginal: j['dateTimeOriginal'] as String?,
        description: j['description'] as String?,
        fileSizeInByte: j['fileSizeInByte'] is int ? j['fileSizeInByte'] as int : (j['fileSizeInByte'] is num ? (j['fileSizeInByte'] as num).toInt() : int.tryParse(j['fileSizeInByte']?.toString() ?? '')),
      );

  Map<String, dynamic> toJson() => {
        'city': city,
        'country': country,
        'make': make,
        'model': model,
        'exifImageWidth': exifImageWidth,
        'exifImageHeight': exifImageHeight,
        'dateTimeOriginal': dateTimeOriginal,
        'description': description,
        'fileSizeInByte': fileSizeInByte,
      };
}

class MetadataSearchResponse {
  MetadataSearchResponse({required this.items, this.total, this.nextPage});
  final List<ImmichAsset> items;
  final int? total;
  final String? nextPage;

  factory MetadataSearchResponse.fromJson(Map<String, dynamic> j) {
    final assets = j['assets'] as Map<String, dynamic>?;
    if (assets == null) return MetadataSearchResponse(items: []);
    final list = (assets['items'] as List? ?? [])
        .map((e) => ImmichAsset.fromJson(e as Map<String, dynamic>))
        .toList();
    return MetadataSearchResponse(
      items: list,
      total: assets['total'] as int?,
      nextPage: assets['nextPage'] as String?,
    );
  }
}
