import 'dart:async';
import 'dart:typed_data';
import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:immich_swipe/core/api_client.dart';
import 'package:immich_swipe/core/secure_store.dart';
import 'package:immich_swipe/data/album_repository.dart';
import 'package:immich_swipe/data/asset_repository.dart';
import 'package:immich_swipe/models/immich_album.dart';
import 'package:immich_swipe/models/immich_asset.dart';
import 'package:immich_swipe/models/session.dart';
import 'package:immich_swipe/state/providers.dart';

// ------------------------------------------------------------------
// Fakes
// ------------------------------------------------------------------

class _FakeSecureStore extends SecureStore {
  bool cleared = false;
  Set<String> reviewed = {};
  Map<String, String?> sessionMap = {};

  _FakeSecureStore({Map<String, String?>? session}) {
    if (session != null) sessionMap = session;
  }

  @override
  Future<Map<String, String?>> readSession() async => sessionMap;
  @override
  Future<void> saveSession({required String serverUrl, required String token, required String userName, required String mode}) async {
    sessionMap = {'serverUrl': serverUrl, 'token': token, 'userName': userName, 'mode': mode};
  }

  @override
  Future<void> clearSession() async => cleared = true;
  @override
  Future<Set<String>> readReviewed(String a, String b) async => Set<String>.from(reviewed);
  @override
  Future<void> writeReviewed(String a, String b, Set<String> ids) async => reviewed = Set<String>.from(ids);
  @override
  Future<void> addReviewed(String a, String b, String id) async => reviewed.add(id);
  @override
  Future<bool> readSkipVideos() async => false;
  @override
  Future<void> writeSkipVideos(bool v) async {}
}

class _FakeAssetRepo extends AssetRepository {
  _FakeAssetRepo() : super(Dio());
  List<String> trashedIds = [];
  List<String> restoredIds = [];
  List<ImmichAsset> Function()? randomFactory;
  List<ImmichAsset> randomReturn = [];
  int searchRandomCalls = 0;
  Future<void> Function(String id)? trashOverride;
  Future<void> Function(String id)? restoreOverride;

  @override
  Future<List<ImmichAsset>> searchRandom({required int size, bool imageOnly = false}) async {
    searchRandomCalls++;
    if (randomFactory != null) return randomFactory!();
    return randomReturn;
  }

  @override
  Future<void> trash(String id) async {
    if (trashOverride != null) return trashOverride!(id);
    trashedIds.add(id);
  }

  @override
  Future<void> restore(String id) async {
    if (restoreOverride != null) return restoreOverride!(id);
    restoredIds.add(id);
  }

  @override
  Future<void> setFavorite(String id, bool v) async {}
  @override
  Future<MetadataSearchResponse> searchMetadata({int page = 1, int size = 50, String order = 'asc', String? assetType}) async => MetadataSearchResponse(items: []);
  @override
  Future<Uint8List> fetchThumbnail(String id, {String size = 'preview'}) async => Uint8List(0);
  @override
  Future<Uint8List> fetchOriginal(String id) async => Uint8List(0);
}

class _FakeAlbumRepo extends AlbumRepository {
  _FakeAlbumRepo() : super(Dio());
  @override
  Future<List<ImmichAlbum>> listAlbums() async => [];
  @override
  Future<void> addToAlbum(String a, String b) async {}
}

ProviderContainer _make({
  _FakeSecureStore? store,
  _FakeAssetRepo? assetRepo,
}) {
  final fakeDio = Dio(BaseOptions(baseUrl: 'http://example.com/api'));
  return ProviderContainer(overrides: [
    dioProvider.overrideWithValue(fakeDio),
    secureStoreProvider.overrideWithValue(store ?? _FakeSecureStore()),
    assetRepositoryProvider.overrideWithValue(assetRepo ?? _FakeAssetRepo()),
    albumRepositoryProvider.overrideWithValue(_FakeAlbumRepo()),
  ]);
}

ImmichAsset _asset(String id, {int? bytes, String type = 'IMAGE'}) => ImmichAsset(
      id: id,
      type: type,
      originalFileName: '$id.jpg',
      fileCreatedAt: '',
      isFavorite: false,
      exifInfo: bytes != null ? ExifInfo(fileSizeInByte: bytes) : null,
    );

class _CountingRepo extends AssetRepository {
  _CountingRepo(this.onSearch) : super(Dio());
  final Future<List<ImmichAsset>> Function(int callIndex) onSearch;
  int calls = 0;
  @override
  Future<List<ImmichAsset>> searchRandom({required int size, bool imageOnly = false}) {
    calls++;
    return onSearch(calls);
  }

  @override
  Future<void> trash(String id) async {}
  @override
  Future<void> restore(String id) async {}
  @override
  Future<void> setFavorite(String id, bool v) async {}
  @override
  Future<MetadataSearchResponse> searchMetadata({int page = 1, int size = 50, String order = 'asc', String? assetType}) async => MetadataSearchResponse(items: []);
  @override
  Future<Uint8List> fetchThumbnail(String id, {String size = 'preview'}) async => Uint8List(0);
  @override
  Future<Uint8List> fetchOriginal(String id) async => Uint8List(0);
}

// ------------------------------------------------------------------
// Tests
// ------------------------------------------------------------------

void main() {
  // ---- 1. handleAuthError / clearSessionLocal ----
  group('handleAuthError / clearSessionLocal', () {
    test('401 clears session locally and signs out', () async {
      final store = _FakeSecureStore(session: {
        'serverUrl': 'http://example.com',
        'token': 'tok',
        'userName': 'alice',
        'mode': 'apiKey',
      });
      final c = _make(store: store);
      addTearDown(c.dispose);
      // seed session state directly
      final notifier = c.read(sessionProvider.notifier);
      notifier.state = SessionState(session: SessionInfo(serverUrl: 'http://example.com', token: 'tok', userName: 'alice', mode: 'apiKey'));
      await notifier.handleAuthError(AppException('unauthorized', 'unauthorized', statusCode: 401));
      expect(store.cleared, isTrue);
      expect(c.read(sessionProvider).session, isNull);
      expect(c.read(sessionProvider).isLoggedIn, isFalse);
    });

    test('428 api_key_required sets needsApiKey and retains session', () async {
      final store = _FakeSecureStore();
      final c = _make(store: store);
      addTearDown(c.dispose);
      final sess = SessionInfo(serverUrl: 'http://example.com', token: 'tok', userName: 'alice', mode: 'apiKey');
      c.read(sessionProvider.notifier).state = SessionState(session: sess);
      await c.read(sessionProvider.notifier).handleAuthError(AppException('api_key_required', 'need key', statusCode: 428));
      expect(store.cleared, isFalse);
      expect(c.read(sessionProvider).session, isNotNull);
      expect(c.read(sessionProvider).needsApiKey, isTrue);
      expect(c.read(sessionProvider).error, 'api_key_required');
    });

    test('handleAuthError with code api_key_required (no status) also sets needsApiKey', () async {
      final store = _FakeSecureStore();
      final c = _make(store: store);
      addTearDown(c.dispose);
      final sess = SessionInfo(serverUrl: 'http://example.com', token: 'tok', userName: 'alice', mode: 'apiKey');
      c.read(sessionProvider.notifier).state = SessionState(session: sess);
      await c.read(sessionProvider.notifier).handleAuthError(AppException('api_key_required', 'need key'));
      expect(c.read(sessionProvider).needsApiKey, isTrue);
      expect(c.read(sessionProvider).session, isNotNull);
    });

    test('clearSessionLocal clears store and signs out', () async {
      final store = _FakeSecureStore(session: {
        'serverUrl': 'http://example.com',
        'token': 'tok',
        'userName': 'alice',
        'mode': 'apiKey',
      });
      final c = _make(store: store);
      addTearDown(c.dispose);
      c.read(sessionProvider.notifier).state = SessionState(session: SessionInfo(serverUrl: 'http://example.com', token: 'tok', userName: 'alice', mode: 'apiKey'));
      await c.read(sessionProvider.notifier).clearSessionLocal();
      expect(store.cleared, isTrue);
      expect(c.read(sessionProvider).session, isNull);
    });
  });

  // ---- 2. trashCurrent ----
  group('trashCurrent', () {
    test('success increments trashed and freedBytes by fileSizeInByte', () async {
      final repo = _FakeAssetRepo();
      final store = _FakeSecureStore(session: {'serverUrl': 'http://example.com', 'token': 'tok', 'userName': 'alice', 'mode': 'apiKey'});
      final c = _make(store: store, assetRepo: repo);
      addTearDown(c.dispose);
      final rn = c.read(reviewProvider.notifier);
      final a = _asset('a1', bytes: 12345);
      rn.state = ReviewState(assets: [a], reviewedIds: {}, history: []);
      await rn.trashCurrent();
      expect(c.read(statsProvider).trashed, 1);
      expect(c.read(statsProvider).freedBytes, 12345);
      expect(repo.trashedIds, ['a1']);
      expect(c.read(reviewProvider).reviewedIds, contains('a1'));
    });

    test('failure rolls back deck, history, reviewedIds and stats; reviewed not persisted', () async {
      final repo = _FakeAssetRepo();
      repo.trashOverride = (id) async => throw AppException('network_error', 'fail', statusCode: 500);
      final store = _FakeSecureStore(session: {'serverUrl': 'http://example.com', 'token': 'tok', 'userName': 'alice', 'mode': 'apiKey'});
      final c = _make(store: store, assetRepo: repo);
      addTearDown(c.dispose);
      final rn = c.read(reviewProvider.notifier);
      final a = _asset('fail1', bytes: 999);
      final b = _asset('b2', bytes: 100);
      rn.state = ReviewState(assets: [a, b], reviewedIds: {'existing'}, history: []);
      // pre-seed stats
      c.read(statsProvider.notifier).incKept();
      final statsBefore = c.read(statsProvider);
      expect(statsBefore.kept, 1);
      try {
        await rn.trashCurrent();
        fail('should throw');
      } catch (_) {}
      // deck restored
      expect(c.read(reviewProvider).assets.length, 2);
      expect(c.read(reviewProvider).assets.first.id, 'fail1');
      // history restored
      expect(c.read(reviewProvider).history, isEmpty);
      // reviewedIds restored (fail1 NOT added)
      expect(c.read(reviewProvider).reviewedIds, isNot(contains('fail1')));
      expect(c.read(reviewProvider).reviewedIds, contains('existing'));
      expect(store.reviewed, isNot(contains('fail1')));
      // stats rolled back
      expect(c.read(statsProvider).kept, 1);
      expect(c.read(statsProvider).trashed, 0);
      expect(c.read(statsProvider).freedBytes, 0);
    });

    test('failure with 401 also rolls back and does not rethrow as visible error (returns)', () async {
      final repo = _FakeAssetRepo();
      repo.trashOverride = (id) async => throw AppException('unauthorized', 'unauthorized', statusCode: 401);
      final store = _FakeSecureStore(session: {'serverUrl': 'http://example.com', 'token': 'tok', 'userName': 'alice', 'mode': 'apiKey'});
      final c = _make(store: store, assetRepo: repo);
      addTearDown(c.dispose);
      // seed session
      c.read(sessionProvider.notifier).state = SessionState(session: SessionInfo(serverUrl: 'http://example.com', token: 'tok', userName: 'alice', mode: 'apiKey'));
      final rn = c.read(reviewProvider.notifier);
      rn.state = ReviewState(assets: [_asset('x1', bytes: 500)], reviewedIds: {}, history: []);
      await rn.trashCurrent(); // should return without throwing for 401 per impl
      expect(c.read(reviewProvider).assets.length, 1);
      expect(c.read(statsProvider).trashed, 0);
      expect(store.cleared, isTrue);
      expect(c.read(sessionProvider).session, isNull);
    });
  });

  // ---- 3. keepCurrent ----
  group('keepCurrent', () {
    test('success leaves reviewedIds containing asset, increments kept, advances deck', () async {
      final store = _FakeSecureStore(session: {'serverUrl': 'http://example.com', 'token': 'tok', 'userName': 'alice', 'mode': 'apiKey'});
      final c = _make(store: store);
      addTearDown(c.dispose);
      final rn = c.read(reviewProvider.notifier);
      final a1 = _asset('k1');
      final a2 = _asset('k2');
      rn.state = ReviewState(assets: [a1, a2], reviewedIds: {}, history: []);
      await rn.keepCurrent();
      expect(c.read(reviewProvider).assets.length, 1);
      expect(c.read(reviewProvider).assets.first.id, 'k2');
      expect(c.read(reviewProvider).reviewedIds, contains('k1'));
      expect(c.read(statsProvider).kept, 1);
      expect(c.read(reviewProvider).history.length, 1);
      expect(c.read(reviewProvider).history.first.type, ReviewActionType.keep);
    });

    test('keepCurrent persists reviewed so asset can not reappear', () async {
      final store = _FakeSecureStore(session: {'serverUrl': 'http://example.com', 'token': 'tok', 'userName': 'alice', 'mode': 'apiKey'});
      final c = _make(store: store);
      addTearDown(c.dispose);
      c.read(sessionProvider.notifier).state = SessionState(session: SessionInfo(serverUrl: 'http://example.com', token: 'tok', userName: 'alice', mode: 'apiKey'));
      await Future.delayed(const Duration(milliseconds: 10));
      final rn = c.read(reviewProvider.notifier);
      rn.state = ReviewState(assets: [_asset('k9')], reviewedIds: {}, history: []);
      await rn.keepCurrent();
      expect(store.reviewed, contains('k9'));
    });
  });

  // ---- 4. undo ----
  group('undo', () {
    test('undo keep removes reviewedId, decrements kept, returns asset to top', () async {
      final store = _FakeSecureStore(session: {'serverUrl': 'http://example.com', 'token': 'tok', 'userName': 'alice', 'mode': 'apiKey'});
      final c = _make(store: store);
      addTearDown(c.dispose);
      final rn = c.read(reviewProvider.notifier);
      final a1 = _asset('a1');
      final a2 = _asset('a2');
      rn.state = ReviewState(assets: [a1, a2], reviewedIds: {}, history: []);
      await rn.keepCurrent();
      expect(c.read(reviewProvider).assets.first.id, 'a2');
      await rn.undo();
      expect(c.read(reviewProvider).assets.first.id, 'a1');
      expect(c.read(reviewProvider).assets.length, 2);
      expect(c.read(reviewProvider).reviewedIds, isNot(contains('a1')));
      expect(c.read(statsProvider).kept, 0);
    });

    test('undo trash issues restore, removes reviewedId, decrements freedBytes, returns asset to top', () async {
      final repo = _FakeAssetRepo();
      final store = _FakeSecureStore(session: {'serverUrl': 'http://example.com', 'token': 'tok', 'userName': 'alice', 'mode': 'apiKey'});
      final c = _make(store: store, assetRepo: repo);
      addTearDown(c.dispose);
      final rn = c.read(reviewProvider.notifier);
      final a1 = _asset('t1', bytes: 777);
      final a2 = _asset('t2');
      rn.state = ReviewState(assets: [a1, a2], reviewedIds: {}, history: []);
      await rn.trashCurrent();
      expect(c.read(statsProvider).freedBytes, 777);
      expect(c.read(statsProvider).trashed, 1);
      await rn.undo();
      expect(repo.restoredIds, ['t1']);
      expect(c.read(reviewProvider).assets.first.id, 't1');
      expect(c.read(reviewProvider).reviewedIds, isNot(contains('t1')));
      expect(c.read(statsProvider).trashed, 0);
      expect(c.read(statsProvider).freedBytes, 0);
    });

    test('undo trash decrements freedBytes clamped at 0', () async {
      final repo = _FakeAssetRepo();
      final store = _FakeSecureStore(session: {'serverUrl': 'http://example.com', 'token': 'tok', 'userName': 'alice', 'mode': 'apiKey'});
      final c = _make(store: store, assetRepo: repo);
      addTearDown(c.dispose);
      final rn = c.read(reviewProvider.notifier);
      final a = _asset('clamp1', bytes: 100);
      rn.state = ReviewState(assets: [a], reviewedIds: {}, history: []);
      await rn.trashCurrent();
      // manually corrupt stats to test clamp
      c.read(statsProvider.notifier).state = StatsState(kept: 0, trashed: 1, freedBytes: 10);
      // history entry still has freedBytes 100, dec should clamp to 0
      await rn.undo();
      expect(c.read(statsProvider).freedBytes, 0);
    });

    test('undo when history empty does nothing', () async {
      final c = _make();
      addTearDown(c.dispose);
      final rn = c.read(reviewProvider.notifier);
      rn.state = ReviewState(assets: [_asset('a1')], reviewedIds: {}, history: []);
      await rn.undo();
      expect(c.read(reviewProvider).assets.length, 1);
    });
  });

  // ---- 5. Deck exhaustion / retry loop ----
  group('deck exhaustion', () {
    test('retries bounded attempts then sets noCandidates when only reviewed assets returned', () async {
      final dup = _asset('dup');
      final repo = _CountingRepo((call) async => [dup]);
      final store = _FakeSecureStore(session: {'serverUrl': 'http://example.com', 'token': 'tok', 'userName': 'alice', 'mode': 'apiKey'});
      store.reviewed = {'dup'};
      final c = ProviderContainer(overrides: [
        dioProvider.overrideWithValue(Dio(BaseOptions(baseUrl: 'http://example.com/api'))),
        secureStoreProvider.overrideWithValue(store),
        assetRepositoryProvider.overrideWithValue(repo),
        albumRepositoryProvider.overrideWithValue(_FakeAlbumRepo()),
      ]);
      addTearDown(c.dispose);
      // ensure SessionNotifier restore completes before loadInitial
      await Future.delayed(const Duration(milliseconds: 20));
      c.read(sessionProvider.notifier).state = SessionState(session: SessionInfo(serverUrl: 'http://example.com', token: 'tok', userName: 'alice', mode: 'apiKey'));
      final rn = c.read(reviewProvider.notifier);
      await rn.loadInitial();
      expect(repo.calls, 20); // _randomMaxAttempts
      expect(repo.calls, greaterThan(1));
      expect(c.read(reviewProvider).noCandidates, isTrue);
      expect(c.read(reviewProvider).assets, isEmpty);
      expect(c.read(reviewProvider).error, isNull);
    });

    test('when candidate eventually found it is added and noCandidates stays false (retry >1)', () async {
      final dup = _asset('dup');
      final fresh = _asset('fresh');
      final repo = _CountingRepo((call) async => call <= 3 ? [dup] : [fresh]);
      final store = _FakeSecureStore(session: {'serverUrl': 'http://example.com', 'token': 'tok', 'userName': 'alice', 'mode': 'apiKey'});
      store.reviewed = {'dup'};
      final c = ProviderContainer(overrides: [
        dioProvider.overrideWithValue(Dio(BaseOptions(baseUrl: 'http://example.com/api'))),
        secureStoreProvider.overrideWithValue(store),
        assetRepositoryProvider.overrideWithValue(repo),
        albumRepositoryProvider.overrideWithValue(_FakeAlbumRepo()),
      ]);
      addTearDown(c.dispose);
      await Future.delayed(const Duration(milliseconds: 20));
      c.read(sessionProvider.notifier).state = SessionState(session: SessionInfo(serverUrl: 'http://example.com', token: 'tok', userName: 'alice', mode: 'apiKey'));
      await c.read(reviewProvider.notifier).loadInitial();
      expect(repo.calls, greaterThan(1));
      expect(c.read(reviewProvider).assets.map((e) => e.id), contains('fresh'));
      expect(c.read(reviewProvider).noCandidates, isFalse);
    });

    test('noCandidates distinct from generic error', () async {
      final repo = _CountingRepo((_) async => throw Exception('down'));
      final c = ProviderContainer(overrides: [
        dioProvider.overrideWithValue(Dio(BaseOptions(baseUrl: 'http://example.com/api'))),
        secureStoreProvider.overrideWithValue(_FakeSecureStore()),
        assetRepositoryProvider.overrideWithValue(repo),
        albumRepositoryProvider.overrideWithValue(_FakeAlbumRepo()),
      ]);
      addTearDown(c.dispose);
      await Future.delayed(const Duration(milliseconds: 20));
      await c.read(reviewProvider.notifier).loadInitial();
      expect(c.read(reviewProvider).loading, isFalse);
      expect(c.read(reviewProvider).error, isNotNull);
      expect(c.read(reviewProvider).noCandidates, isFalse);
    });

    test('successful loadInitial leaves error null', () async {
      final repo = _CountingRepo((_) async => [_asset('ok1')]);
      final c = ProviderContainer(overrides: [
        dioProvider.overrideWithValue(Dio(BaseOptions(baseUrl: 'http://example.com/api'))),
        secureStoreProvider.overrideWithValue(_FakeSecureStore()),
        assetRepositoryProvider.overrideWithValue(repo),
        albumRepositoryProvider.overrideWithValue(_FakeAlbumRepo()),
      ]);
      addTearDown(c.dispose);
      await Future.delayed(const Duration(milliseconds: 20));
      await c.read(reviewProvider.notifier).loadInitial();
      expect(c.read(reviewProvider).loading, isFalse);
      expect(c.read(reviewProvider).error, isNull);
      expect(c.read(reviewProvider).noCandidates, isFalse);
      expect(c.read(reviewProvider).assets.map((e) => e.id), contains('ok1'));
    });
  });

  // ---- 6. Epoch guard ----
  group('epoch guard', () {
    test('stale fetch after epoch bump is dropped and does not mutate deck', () async {
      // Use keepCurrent's unawaited _fetchMore (epoch captured) + loadInitial bump.
      // Completer controls the stale fetch; subsequent calls return fresh or empty.
      final staleCompleter = Completer<List<ImmichAsset>>();
      final freshAsset = _asset('fresh_after_bump');
      final staleAsset = _asset('stale1');
      int call = 0;
      final repo = _CountingRepo((idx) async {
        call++;
        if (call == 1) return staleCompleter.future; // keepCurrent's fetch
        // loadInitial's fetch attempts: return fresh on first attempt then empty to stop loop
        if (call == 2) return [freshAsset];
        return [];
      });
      final store = _FakeSecureStore(session: {'serverUrl': 'http://example.com', 'token': 'tok', 'userName': 'alice', 'mode': 'apiKey'});
      final c = ProviderContainer(overrides: [
        dioProvider.overrideWithValue(Dio(BaseOptions(baseUrl: 'http://example.com/api'))),
        secureStoreProvider.overrideWithValue(store),
        assetRepositoryProvider.overrideWithValue(repo),
        albumRepositoryProvider.overrideWithValue(_FakeAlbumRepo()),
      ]);
      addTearDown(c.dispose);
      final rn = c.read(reviewProvider.notifier);
      // seed deck with one asset so keepCurrent has something to act on
      rn.state = ReviewState(assets: [_asset('cur')], reviewedIds: {}, history: []);
      // keepCurrent will optimistically remove cur and start _fetchMore(need:5, epoch:0) pending on staleCompleter
      final keepFut = rn.keepCurrent();
      await keepFut; // keepCurrent itself completes after optimistic + addReviewed, but its _fetchMore is unawaited
      // deck now empty (cur removed)
      expect(c.read(reviewProvider).assets, isEmpty);
      // bump epoch via loadInitial which sets _epoch++ and starts its own fetch (call 2)
      final loadFut = rn.loadInitial();
      // loadInitial should complete using freshAsset
      await loadFut;
      expect(c.read(reviewProvider).assets.map((e) => e.id), contains('fresh_after_bump'));
      // now resolve stale fetch — should be dropped
      staleCompleter.complete([staleAsset]);
      // give event loop a tick for stale fetch to resume
      await Future.delayed(const Duration(milliseconds: 20));
      expect(c.read(reviewProvider).assets.map((e) => e.id), isNot(contains('stale1')));
      expect(c.read(reviewProvider).assets.map((e) => e.id), contains('fresh_after_bump'));
    });

    test('non-stale fetch does mutate deck', () async {
      final repo = _CountingRepo((_) async => [_asset('fresh1')]);
      final store = _FakeSecureStore(session: {'serverUrl': 'http://example.com', 'token': 'tok', 'userName': 'alice', 'mode': 'apiKey'});
      final c = ProviderContainer(overrides: [
        dioProvider.overrideWithValue(Dio(BaseOptions(baseUrl: 'http://example.com/api'))),
        secureStoreProvider.overrideWithValue(store),
        assetRepositoryProvider.overrideWithValue(repo),
        albumRepositoryProvider.overrideWithValue(_FakeAlbumRepo()),
      ]);
      addTearDown(c.dispose);
      await c.read(reviewProvider.notifier).loadInitial();
      expect(c.read(reviewProvider).assets.map((e) => e.id), contains('fresh1'));
    });
  });
}
