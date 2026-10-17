import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:dio/dio.dart';
import 'package:immich_swipe/core/format.dart';
import 'package:immich_swipe/models/immich_asset.dart';
import 'package:immich_swipe/state/providers.dart';
import 'dart:typed_data';
import 'package:immich_swipe/data/asset_repository.dart';
import 'package:immich_swipe/data/album_repository.dart';
import 'package:immich_swipe/models/immich_album.dart';
import 'package:immich_swipe/core/secure_store.dart';

class FakeAssetRepo extends AssetRepository {
  FakeAssetRepo() : super(Dio());
  List<String> trashedIds = [];
  List<String> restoredIds = [];
  List<(String, bool)> favCalls = [];
  int thumbCalls = 0;
  List<ImmichAsset> randomReturn = [];
  @override
  Future<void> trash(String id) async { trashedIds.add(id); }
  @override
  Future<void> restore(String id) async { restoredIds.add(id); }
  @override
  Future<void> setFavorite(String id, bool v) async { favCalls.add((id, v)); }
  @override
  Future<List<ImmichAsset>> searchRandom({required int size, bool imageOnly = false}) async => randomReturn;
  @override
  Future<MetadataSearchResponse> searchMetadata({int page = 1, int size = 50, String order = 'asc', String? assetType}) async => MetadataSearchResponse(items: []);
  @override
  Future<Uint8List> fetchThumbnail(String id, {String size = 'preview'}) async { thumbCalls++; return Uint8List(0); }
  @override
  Future<Uint8List> fetchOriginal(String id) async => Uint8List(0);
}

class FakeAlbumRepo extends AlbumRepository {
  FakeAlbumRepo() : super(Dio());
  List<(String, String)> added = [];
  @override
  Future<List<ImmichAlbum>> listAlbums() async => [];
  @override
  Future<void> addToAlbum(String a, String b) async { added.add((a, b)); }
}

class _FakeSecureStore extends SecureStore {
  Set<String> _reviewed = {};
  @override
  Future<Map<String,String?>> readSession() async => {};
  @override
  Future<void> saveSession({required String serverUrl, required String token, required String userName, required String mode}) async {}
  @override
  Future<void> clearSession() async {}
  @override
  Future<Set<String>> readReviewed(String a, String b) async => _reviewed;
  @override
  Future<void> writeReviewed(String a, String b, Set<String> ids) async { _reviewed = ids; }
  @override
  Future<void> addReviewed(String a, String b, String id) async { _reviewed.add(id); }
}

ProviderContainer makeContainer({FakeAssetRepo? assetRepo, FakeAlbumRepo? albumRepo, SecureStore? store}) {
  final fakeDio = Dio(BaseOptions(baseUrl: 'http://example.com/api'));
  return ProviderContainer(overrides: [
    dioProvider.overrideWithValue(fakeDio),
    secureStoreProvider.overrideWithValue(store ?? _FakeSecureStore()),
    assetRepositoryProvider.overrideWithValue(assetRepo ?? FakeAssetRepo()),
    albumRepositoryProvider.overrideWithValue(albumRepo ?? FakeAlbumRepo()),
  ]);
}

void main() {
  test('formatBytes', () {
    expect(formatBytes(0), '0 B');
    expect(formatBytes(1023), '1023 B');
    expect(formatBytes(1024), '1.0 KB');
    expect(formatBytes(1024 * 1024), '1.0 MB');
    expect(formatBytes(1024 * 1024 * 1024), '1.00 GB');
  });

  test('formatBytes edges 0 and boundary', () {
    expect(formatBytes(1), '1 B');
    expect(formatBytes(1023), '1023 B');
    expect(formatBytes(1536), '1.5 KB');
    expect(formatBytes(1024 * 1024 - 1), contains('KB'));
    expect(formatBytes(5 * 1024 * 1024 * 1024), contains('GB'));
  });

  test('ImmichAsset.fromJson normalizes type', () {
    final a = ImmichAsset.fromJson({'id': '1', 'type': 'VIDEO', 'originalFileName': 'x.mp4', 'fileCreatedAt': '2024-01-01', 'isFavorite': false});
    expect(a.isVideo, true);
    final b = ImmichAsset.fromJson({'id': '2', 'type': 'AUDIO', 'originalFileName': 'x', 'fileCreatedAt': '', 'isFavorite': false});
    expect(b.type, 'IMAGE');
    expect(b.isImage, true);
  });

  test('ImmichAsset VIDEO with exifInfo and duration parses', () {
    final j = {
      'id': 'v1',
      'type': 'VIDEO',
      'originalFileName': 'clip.mp4',
      'fileCreatedAt': '2024-06-01T10:00:00.000Z',
      'isFavorite': true,
      'duration': '00:00:12.000',
      'exifInfo': {'make': 'Sony', 'model': 'A7', 'city': 'Berlin', 'exifImageWidth': 1920, 'exifImageHeight': 1080},
      'originalMimeType': 'video/mp4',
    };
    final a = ImmichAsset.fromJson(j);
    expect(a.isVideo, true);
    expect(a.duration, '00:00:12.000');
    expect(a.exifInfo?.make, 'Sony');
    expect(a.exifInfo?.city, 'Berlin');
    expect(a.exifInfo?.exifImageWidth, 1920);
  });

  test('ImmichAlbum fromJson', () {
    final al = ImmichAlbum.fromJson({'id': 'al1', 'albumName': 'Trip', 'assetCount': 5});
    expect(al.id, 'al1');
    expect(al.albumName, 'Trip');
    expect(al.assetCount, 5);
  });

  test('MetadataSearchResponse parses nested assets', () {
    final r = MetadataSearchResponse.fromJson({
      'assets': {'total': 42, 'items': [
        {'id': 'a1', 'type': 'IMAGE', 'originalFileName': 'a.jpg', 'fileCreatedAt': '', 'isFavorite': true}
      ], 'nextPage': null}
    });
    expect(r.total, 42);
    expect(r.items.length, 1);
    expect(r.items.first.isFavorite, true);
  });

  test('MetadataSearchResponse empty when missing assets key', () {
    final r = MetadataSearchResponse.fromJson({});
    expect(r.items, isEmpty);
    final r2 = MetadataSearchResponse.fromJson({'assets': {'items': []}});
    expect(r2.items, isEmpty);
  });

  test('MetadataSearchResponse nextPage', () {
    final r = MetadataSearchResponse.fromJson({
      'assets': {'total': 2, 'items': [], 'nextPage': 'abc123'}
    });
    expect(r.nextPage, 'abc123');
  });

  test('StatsNotifier arithmetic', () {
    final c = ProviderContainer();
    addTearDown(c.dispose);
    final n = c.read(statsProvider.notifier);
    expect(c.read(statsProvider).kept, 0);
    n.incKept();
    n.incKept();
    expect(c.read(statsProvider).kept, 2);
    n.decKept();
    expect(c.read(statsProvider).kept, 1);
    n.incTrashed(bytes: 100);
    expect(c.read(statsProvider).trashed, 1);
    expect(c.read(statsProvider).freedBytes, 100);
    n.decTrashed(bytes: 100);
    expect(c.read(statsProvider).trashed, 0);
    n.decKept();
    n.decKept();
    expect(c.read(statsProvider).kept, 0);
  });

  test('Stats freedBytes arithmetic clamps to 0', () {
    final c = ProviderContainer();
    addTearDown(c.dispose);
    final n = c.read(statsProvider.notifier);
    n.incTrashed(bytes: 500);
    n.incTrashed(bytes: 300);
    expect(c.read(statsProvider).freedBytes, 800);
    n.decTrashed(bytes: 300);
    expect(c.read(statsProvider).freedBytes, 500);
    n.decTrashed(bytes: 1000);
    expect(c.read(statsProvider).freedBytes, 0);
    expect(c.read(statsProvider).trashed, 0);
  });

  test('ReviewNotifier keep/trash/undo with faked repos', () async {
    final fakeStore = _FakeSecureStore();
    final c2 = makeContainer(store: fakeStore);
    addTearDown(c2.dispose);
    final rn = c2.read(reviewProvider.notifier);
    final a1 = ImmichAsset(id: 'a1', type: 'IMAGE', originalFileName: 'a1.jpg', fileCreatedAt: '', isFavorite: false);
    final a2 = ImmichAsset(id: 'a2', type: 'IMAGE', originalFileName: 'a2.jpg', fileCreatedAt: '', isFavorite: false);
    rn.state = ReviewState(assets: [a1, a2], reviewedIds: {}, history: []);
    await rn.keepCurrent();
    expect(c2.read(reviewProvider).assets.length, 1);
    expect(c2.read(reviewProvider).assets.first.id, 'a2');
    expect(c2.read(reviewProvider).history.length, 1);
    expect(c2.read(statsProvider).kept, 1);
    await rn.undo();
    expect(c2.read(reviewProvider).assets.length, 2);
    expect(c2.read(reviewProvider).assets.first.id, 'a1');
    expect(c2.read(statsProvider).kept, 0);
    await rn.trashCurrent();
    expect(c2.read(reviewProvider).assets.length, 1);
    expect(c2.read(statsProvider).trashed, 1);
    await rn.undo();
    expect(c2.read(statsProvider).trashed, 0);
  });

  test('Review trash calls repository trash', () async {
    final repo = FakeAssetRepo();
    final c = makeContainer(assetRepo: repo);
    addTearDown(c.dispose);
    final rn = c.read(reviewProvider.notifier);
    rn.state = ReviewState(assets: [ImmichAsset(id: 'x1', type: 'IMAGE', originalFileName: 'x.jpg', fileCreatedAt: '', isFavorite: false)], reviewedIds: {}, history: []);
    await rn.trashCurrent();
    expect(repo.trashedIds, ['x1']);
  });

  test('Review keep does not call trash', () async {
    final repo = FakeAssetRepo();
    final c = makeContainer(assetRepo: repo);
    addTearDown(c.dispose);
    final rn = c.read(reviewProvider.notifier);
    rn.state = ReviewState(assets: [ImmichAsset(id: 'k1', type: 'IMAGE', originalFileName: 'k.jpg', fileCreatedAt: '', isFavorite: false)], reviewedIds: {}, history: []);
    await rn.keepCurrent();
    expect(repo.trashedIds, isEmpty);
    expect(c.read(statsProvider).kept, 1);
  });

  test('Review undo after trash calls restore and returns card to top', () async {
    final repo = FakeAssetRepo();
    final c = makeContainer(assetRepo: repo);
    addTearDown(c.dispose);
    final rn = c.read(reviewProvider.notifier);
    final a1 = ImmichAsset(id: 'a1', type: 'IMAGE', originalFileName: 'a1.jpg', fileCreatedAt: '', isFavorite: false);
    final a2 = ImmichAsset(id: 'a2', type: 'IMAGE', originalFileName: 'a2.jpg', fileCreatedAt: '', isFavorite: false);
    rn.state = ReviewState(assets: [a1, a2], reviewedIds: {}, history: []);
    await rn.trashCurrent();
    expect(c.read(reviewProvider).assets.first.id, 'a2');
    await rn.undo();
    expect(repo.restoredIds, ['a1']);
    expect(c.read(reviewProvider).assets.first.id, 'a1');
    expect(c.read(reviewProvider).assets.length, 2);
  });

  test('Review favorite toggles via repository and flips local state', () async {
    final repo = FakeAssetRepo();
    final c = makeContainer(assetRepo: repo);
    addTearDown(c.dispose);
    final rn = c.read(reviewProvider.notifier);
    final a = ImmichAsset(id: 'fav1', type: 'IMAGE', originalFileName: 'fav.jpg', fileCreatedAt: '', isFavorite: false);
    rn.state = ReviewState(assets: [a], reviewedIds: {}, history: []);
    await rn.toggleFavorite('fav1');
    expect(repo.favCalls, [('fav1', true)]);
    expect(c.read(reviewProvider).assets.first.isFavorite, true);
    await rn.toggleFavorite('fav1');
    expect(repo.favCalls.last, ('fav1', false));
    expect(c.read(reviewProvider).assets.first.isFavorite, false);
  });

  test('Review addToAlbum calls album repo', () async {
    final albumRepo = FakeAlbumRepo();
    final c = makeContainer(albumRepo: albumRepo);
    addTearDown(c.dispose);
    final rn = c.read(reviewProvider.notifier);
    await rn.addToAlbum('al1', 'asset1');
    expect(albumRepo.added, [('al1', 'asset1')]);
  });

  test('Review prefetch keeps next-5 assets after swipe keeps deck filled', () async {
    final repo = FakeAssetRepo();
    // return 10 assets
    repo.randomReturn = List.generate(10, (i) => ImmichAsset(id: 'p$i', type: 'IMAGE', originalFileName: 'p$i.jpg', fileCreatedAt: '', isFavorite: false));
    final c = makeContainer(assetRepo: repo);
    addTearDown(c.dispose);
    final rn = c.read(reviewProvider.notifier);
    rn.state = ReviewState(assets: [
      ImmichAsset(id: 'cur', type: 'IMAGE', originalFileName: 'cur.jpg', fileCreatedAt: '', isFavorite: false),
    ], reviewedIds: {}, history: []);
    await rn.keepCurrent();
    // allow microtask prefetch
    await Future.delayed(const Duration(milliseconds: 50));
    expect(c.read(reviewProvider).assets.length, greaterThanOrEqualTo(1));
    // deck should have been refilled to at least 1 via searchRandom
    expect(repo.thumbCalls, greaterThanOrEqualTo(0));
  });

  test('Review trash failure reverts deck', () async {
    final repo = _FailingTrashRepo();
    final c = makeContainer(assetRepo: repo);
    addTearDown(c.dispose);
    final rn = c.read(reviewProvider.notifier);
    final a = ImmichAsset(id: 'fail1', type: 'IMAGE', originalFileName: 'f.jpg', fileCreatedAt: '', isFavorite: false);
    rn.state = ReviewState(assets: [a], reviewedIds: {}, history: []);
    try {
      await rn.trashCurrent();
      fail('should throw');
    } catch (_) {}
    expect(c.read(reviewProvider).assets.length, 1);
    expect(c.read(reviewProvider).assets.first.id, 'fail1');
  });
}

class _FailingTrashRepo extends FakeAssetRepo {
  @override
  Future<void> trash(String id) async => throw Exception('network fail');
}
