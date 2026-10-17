class ImmichAlbum {
  ImmichAlbum({
    required this.id,
    required this.albumName,
    this.assetCount,
    this.albumThumbnailAssetId,
  });
  final String id;
  final String albumName;
  final int? assetCount;
  final String? albumThumbnailAssetId;

  factory ImmichAlbum.fromJson(Map<String, dynamic> j) => ImmichAlbum(
        id: j['id'] as String,
        albumName: j['albumName'] as String? ?? '',
        assetCount: j['assetCount'] as int?,
        albumThumbnailAssetId: j['albumThumbnailAssetId'] as String?,
      );
}

class ImmichPerson {
  ImmichPerson({required this.id, required this.name});
  final String id;
  final String name;
  factory ImmichPerson.fromJson(Map<String, dynamic> j) =>
      ImmichPerson(id: j['id'] as String, name: j['name'] as String? ?? '');
}
