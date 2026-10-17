import 'dart:convert';
import 'dart:typed_data';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:immich_swipe/features/review/widgets/stats_bar.dart';
import 'package:immich_swipe/features/review/widgets/album_chips.dart';
import 'package:immich_swipe/features/review/widgets/swipe_card.dart';
import 'package:immich_swipe/features/review/review_screen.dart';
import 'package:immich_swipe/features/auth/login_screen.dart';
import 'package:immich_swipe/models/immich_album.dart';
import 'package:immich_swipe/models/immich_asset.dart';
import 'package:immich_swipe/state/providers.dart';
import 'package:immich_swipe/data/asset_repository.dart';
import 'package:immich_swipe/data/album_repository.dart';
import 'package:immich_swipe/core/secure_store.dart';
import 'package:dio/dio.dart';

final _tinyPng = base64Decode('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+ip1sAAAAASUVORK5CYII=');

class _FakeAsset extends AssetRepository {
  _FakeAsset() : super(Dio());
  @override Future<List<ImmichAsset>> searchRandom({required int size, bool imageOnly = false}) async => [];
  @override Future<Uint8List> fetchThumbnail(String id, {String size = 'preview'}) async => throw Exception('no thumb');
}
/// Records thumbnail fetches so we can assert the deck actually prefetches
/// images after an ASYNC deck load (regression for the "stuck on Loading…" bug).
class _RecordingAsset extends AssetRepository {
  _RecordingAsset() : super(Dio());
  int thumbCalls = 0;
  List<String> thumbIds = [];
  @override Future<List<ImmichAsset>> searchRandom({required int size, bool imageOnly = false}) async => [];
  @override Future<Uint8List> fetchThumbnail(String id, {String size = 'preview'}) async {
    thumbCalls++;
    thumbIds.add(id);
    return _tinyPng;
  }
}
class _FakeAlbum extends AlbumRepository {
  _FakeAlbum() : super(Dio());
  List<(String,String)> added = [];
  @override Future<List<ImmichAlbum>> listAlbums() async => [];
  @override Future<void> addToAlbum(String a, String b) async { added.add((a,b)); }
}
class _FakeStore extends SecureStore {
  @override Future<Map<String,String?>> readSession() async => {};
  @override Future<void> saveSession({required String serverUrl, required String token, required String userName, required String mode}) async {}
  @override Future<void> clearSession() async {}
  @override Future<Set<String>> readReviewed(String a,String b) async => {};
  @override Future<void> writeReviewed(String a,String b, Set<String> ids) async {}
  @override Future<void> addReviewed(String a,String b,String id) async {}
}

Dio _stubDio() {
  final d = Dio(BaseOptions(baseUrl: 'http://example.com/api'));
  d.interceptors.add(InterceptorsWrapper(onRequest: (o, h) {
    if (o.path.contains('/auth/config')) {
      h.resolve(Response(requestOptions: o, statusCode: 200, data: {'oauthEnabled': false, 'oauthButtonText': 'SSO'}));
    } else if (o.path.contains('/auth/login')) {
      h.reject(DioException(requestOptions: o, response: Response(requestOptions: o, statusCode: 401, data: {'message': 'invalid_password', 'code': 'invalid_password'}), type: DioExceptionType.badResponse));
    } else {
      h.resolve(Response(requestOptions: o, statusCode: 200, data: {}));
    }
  }));
  return d;
}

void main() {
  testWidgets('StatsBar copy Kept/Trashed/Freed', (t) async {
    await t.pumpWidget(const MaterialApp(home: Scaffold(body: StatsBar(kept: 7, trashed: 3, freedBytes: 1024))));
    expect(find.text('Kept: 7'), findsOneWidget);
    expect(find.text('Trashed: 3'), findsOneWidget);
    expect(find.textContaining('Freed:'), findsOneWidget);
  });

  testWidgets('AlbumChips renders 10 ActionChips and tapping index N calls onSelect', (t) async {
    final albums = List.generate(12, (i) => ImmichAlbum(id: 'a$i', albumName: 'Album $i'));
    ImmichAlbum? selected;
    await t.pumpWidget(MaterialApp(home: Scaffold(body: AlbumChips(albums: albums, onSelect: (a) => selected = a))));
    expect(find.byType(ActionChip), findsNWidgets(10));
    await t.tap(find.text('Album 3'));
    await t.pump(const Duration(milliseconds: 100));
    expect(selected?.id, 'a3');
    await t.pump(const Duration(milliseconds: 1500));
  });

  testWidgets('AlbumChips shows confirmation check after tap', (t) async {
    final albums = [ImmichAlbum(id: 'a0', albumName: 'Trip')];
    await t.pumpWidget(MaterialApp(home: Scaffold(body: AlbumChips(albums: albums, onSelect: (_) {}))));
    await t.tap(find.byType(ActionChip));
    await t.pump();
    expect(find.byIcon(Icons.check_circle), findsOneWidget);
    await t.pump(const Duration(milliseconds: 1500));
  });

  testWidgets('SwipeCard renders placeholder then imageBytes', (t) async {
    final asset = ImmichAsset(id: '1', type: 'IMAGE', originalFileName: 'photo.jpg', fileCreatedAt: '2024-01-15T10:00:00Z', isFavorite: false);
    await t.pumpWidget(MaterialApp(home: Scaffold(body: SwipeCard(asset: asset))));
    expect(find.text('Loading…'), findsOneWidget);
    await t.pumpWidget(MaterialApp(home: Scaffold(body: SwipeCard(asset: asset, imageBytes: _tinyPng))));
    await t.pump();
    expect(find.byType(Image), findsOneWidget);
  });

  testWidgets('SwipeCard favorite toggle callback', (t) async {
    bool tapped = false;
    final asset = ImmichAsset(id: '1', type: 'IMAGE', originalFileName: 'a.jpg', fileCreatedAt: '', isFavorite: false);
    await t.pumpWidget(MaterialApp(home: Scaffold(body: SwipeCard(asset: asset, onFavoriteToggle: () => tapped = true))));
    await t.tap(find.byIcon(Icons.favorite_border_rounded));
    expect(tapped, true);
  });

  testWidgets('SwipeCard shows favorite indicator when isFavorite', (t) async {
    final asset = ImmichAsset(id: '1', type: 'IMAGE', originalFileName: 'a.jpg', fileCreatedAt: '', isFavorite: true);
    await t.pumpWidget(MaterialApp(home: Scaffold(body: SwipeCard(asset: asset))));
    expect(find.byIcon(Icons.favorite_rounded), findsWidgets);
  });

  testWidgets('SwipeCard info button triggers onInfo (EXIF sheet)', (t) async {
    bool info = false;
    final asset = ImmichAsset(id: '1', type: 'IMAGE', originalFileName: 'a.jpg', fileCreatedAt: '2024-01-01', isFavorite: false, exifInfo: ExifInfo(make: 'Canon', model: 'R5'));
    await t.pumpWidget(MaterialApp(home: Scaffold(body: SwipeCard(asset: asset, onInfo: () => info = true))));
    await t.tap(find.byIcon(Icons.info_outline_rounded));
    expect(info, true);
  });

  testWidgets('ReviewScreen completion all caught up state', (t) async {
    final dio = _stubDio();
    final container = ProviderContainer(overrides: [
      dioProvider.overrideWithValue(dio),
      assetRepositoryProvider.overrideWithValue(_FakeAsset()),
      albumRepositoryProvider.overrideWithValue(_FakeAlbum()),
      secureStoreProvider.overrideWithValue(_FakeStore()),
    ]);
    addTearDown(container.dispose);
    await t.pumpWidget(UncontrolledProviderScope(container: container, child: const MaterialApp(home: ReviewScreen())));
    await t.pump();
    await t.pump(const Duration(milliseconds: 100));
    // force empty state after loadInitial completes
    container.read(reviewProvider.notifier).state = ReviewState(assets: [], reviewedIds: {}, history: []);
    await t.pump();
    expect(find.text('All caught up'), findsOneWidget);
    expect(find.text('Reload'), findsOneWidget);
  });

  testWidgets('ReviewScreen error/retry state', (t) async {
    final dio = _stubDio();
    final container = ProviderContainer(overrides: [
      dioProvider.overrideWithValue(dio),
      assetRepositoryProvider.overrideWithValue(_FakeAsset()),
      albumRepositoryProvider.overrideWithValue(_FakeAlbum()),
      secureStoreProvider.overrideWithValue(_FakeStore()),
    ]);
    addTearDown(container.dispose);
    await t.pumpWidget(UncontrolledProviderScope(container: container, child: const MaterialApp(home: ReviewScreen())));
    await t.pump();
    await t.pump(const Duration(milliseconds: 100));
    container.read(reviewProvider.notifier).state = ReviewState(assets: [], error: 'network down', reviewedIds: {}, history: []);
    await t.pump();
    expect(find.text('Couldn’t load photos'), findsOneWidget);
    expect(find.text('Try again'), findsOneWidget);
    expect(find.text('network down'), findsWidgets);
  });

  testWidgets('ReviewScreen loading state', (t) async {
    final dio = _stubDio();
    final container = ProviderContainer(overrides: [
      dioProvider.overrideWithValue(dio),
      assetRepositoryProvider.overrideWithValue(_FakeAsset()),
      albumRepositoryProvider.overrideWithValue(_FakeAlbum()),
      secureStoreProvider.overrideWithValue(_FakeStore()),
    ]);
    addTearDown(container.dispose);
    await t.pumpWidget(UncontrolledProviderScope(container: container, child: const MaterialApp(home: ReviewScreen())));
    // immediately set loading before microtask completes
    container.read(reviewProvider.notifier).state = ReviewState(assets: [], loading: true, reviewedIds: {}, history: []);
    await t.pump();
    await t.pump(const Duration(milliseconds: 10));
    expect(find.text('Loading photos…'), findsOneWidget);
  });

  testWidgets('ReviewScreen EXIF bottom sheet opens via info button', (t) async {
    final dio = _stubDio();
    final container = ProviderContainer(overrides: [
      dioProvider.overrideWithValue(dio),
      assetRepositoryProvider.overrideWithValue(_FakeAsset()),
      albumRepositoryProvider.overrideWithValue(_FakeAlbum()),
      secureStoreProvider.overrideWithValue(_FakeStore()),
    ]);
    addTearDown(container.dispose);
    final asset = ImmichAsset(id: 'e1', type: 'IMAGE', originalFileName: 'pic.jpg', fileCreatedAt: '2024-03-01T00:00:00Z', isFavorite: false, exifInfo: ExifInfo(make: 'Apple', model: 'iPhone', city: 'Paris'));
    await t.pumpWidget(UncontrolledProviderScope(container: container, child: const MaterialApp(home: ReviewScreen())));
    await t.pump(const Duration(milliseconds: 100));
    container.read(reviewProvider.notifier).state = ReviewState(assets: [asset], reviewedIds: {}, history: []);
    await t.pump();
    await t.tap(find.byIcon(Icons.info_outline_rounded).first);
    await t.pump();
    await t.pump(const Duration(milliseconds: 300));
    expect(find.text('File'), findsOneWidget);
  });

  testWidgets('ReviewScreen prefetches thumbnails after async deck load (regression)', (t) async {
    final repo = _RecordingAsset();
    final container = ProviderContainer(overrides: [
      dioProvider.overrideWithValue(_stubDio()),
      assetRepositoryProvider.overrideWithValue(repo),
      albumRepositoryProvider.overrideWithValue(_FakeAlbum()),
      secureStoreProvider.overrideWithValue(_FakeStore()),
    ]);
    addTearDown(container.dispose);
    await t.pumpWidget(UncontrolledProviderScope(container: container, child: const MaterialApp(home: ReviewScreen())));
    await t.pump(const Duration(milliseconds: 100));
    // deck still empty -> nothing prefetched yet
    expect(repo.thumbCalls, 0);
    final asset = ImmichAsset(id: 't1', type: 'IMAGE', originalFileName: 'x.jpg', fileCreatedAt: '2024-01-01T00:00:00Z', isFavorite: false);
    container.read(reviewProvider.notifier).state = ReviewState(assets: [asset], reviewedIds: {}, history: []);
    await t.pump();
    await t.pump(const Duration(milliseconds: 200));
    // the deck arrived asynchronously: images MUST be requested now
    expect(repo.thumbCalls, greaterThanOrEqualTo(1));
    expect(repo.thumbIds, contains('t1'));
    expect(find.byType(Image), findsWidgets);
  });

  testWidgets('LoginScreen renders form and toggles create mode', (t) async {
    final oldOnError = FlutterError.onError;
    FlutterError.onError = (d) { if (!d.toString().contains('overflowed')) oldOnError?.call(d); };
    addTearDown(() => FlutterError.onError = oldOnError);
    final dio = _stubDio();
    final container = ProviderContainer(overrides: [
      dioProvider.overrideWithValue(dio),
      secureStoreProvider.overrideWithValue(_FakeStore()),
    ]);
    addTearDown(container.dispose);
    await t.pumpWidget(UncontrolledProviderScope(container: container, child: const MaterialApp(home: LoginScreen())));
    await t.pump();
    await t.pump(const Duration(milliseconds: 50));
    expect(find.text('Server URL'), findsOneWidget);
    expect(find.text('User name'), findsOneWidget);
    expect(find.text('Password'), findsOneWidget);
    await t.tap(find.text('Create account'));
    await t.pump();
    expect(find.text('Immich API Key (optional)'), findsOneWidget);
  });

  testWidgets('LoginScreen shows error display container when error set via interaction', (t) async {
    final oldOnError = FlutterError.onError;
    FlutterError.onError = (d) { if (!d.toString().contains('overflowed')) oldOnError?.call(d); };
    addTearDown(() => FlutterError.onError = oldOnError);
    final dio = _stubDio();
    final container = ProviderContainer(overrides: [
      dioProvider.overrideWithValue(dio),
      secureStoreProvider.overrideWithValue(_FakeStore()),
    ]);
    addTearDown(container.dispose);
    await t.pumpWidget(UncontrolledProviderScope(container: container, child: const MaterialApp(home: LoginScreen())));
    await t.pump();
    await t.pump(const Duration(milliseconds: 50));
    await t.enterText(find.widgetWithText(TextField, 'User name'), 'alice');
    await t.enterText(find.widgetWithText(TextField, 'Password'), 'secret123');
    await t.tap(find.widgetWithText(FilledButton, 'Sign in'));
    await t.pumpAndSettle();
    expect(find.byIcon(Icons.error_outline_rounded), findsOneWidget);
  });
}
