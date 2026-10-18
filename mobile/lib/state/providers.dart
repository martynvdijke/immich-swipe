import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:dio/dio.dart';
import '../core/api_client.dart';
import '../core/secure_store.dart';
import '../data/auth_repository.dart';
import '../data/asset_repository.dart';
import '../data/album_repository.dart';
import '../models/immich_asset.dart';
import '../models/immich_album.dart';
import '../models/session.dart';

// ------------------------------------------------------------------
// Core
// ------------------------------------------------------------------
final secureStoreProvider = Provider<SecureStore>((ref) => SecureStore());

class SessionState {
  SessionState({this.session, this.oauthConfig, this.loading = false, this.error, this.needsApiKey = false});
  final SessionInfo? session;
  final AuthConfig? oauthConfig;
  final bool loading;
  final String? error;
  final bool needsApiKey;
  bool get isLoggedIn => session != null;
  static const _unset = Object();
  SessionState copyWith({SessionInfo? session, AuthConfig? oauthConfig, bool? loading, Object? error = _unset, bool? needsApiKey}) =>
      SessionState(session: session ?? this.session, oauthConfig: oauthConfig ?? this.oauthConfig, loading: loading ?? this.loading, error: error == _unset ? this.error : error as String?, needsApiKey: needsApiKey ?? this.needsApiKey);
}

class SessionNotifier extends StateNotifier<SessionState> {
  SessionNotifier(this.ref) : super(SessionState()) {
    _restore();
  }
  final Ref ref;

  Dio _dioFor(String serverUrl, String? token) =>
      ApiClient(serverUrl: serverUrl, token: token).dio;

  Future<void> _restore() async {
    final s = await ref.read(secureStoreProvider).readSession();
    final url = s['serverUrl'];
    final token = s['token'];
    final user = s['userName'];
    final mode = s['mode'];
    if (url != null && token != null && user != null) {
      state = SessionState(
          session: SessionInfo(serverUrl: url, token: token, userName: user, mode: mode ?? 'apiKey'),
          oauthConfig: state.oauthConfig);
      await fetchConfig();
    } else {
      await fetchConfigFor(null);
    }
  }

  Future<void> fetchConfig() async {
    await fetchConfigFor(state.session?.serverUrl);
  }

  Future<void> fetchConfigFor(String? serverUrl) async {
    try {
      final dio = _dioFor(serverUrl ?? 'http://localhost:8080', state.session?.token);
      final repo = AuthRepository(dio);
      final cfg = await repo.getConfig();
      state = SessionState(session: state.session, oauthConfig: cfg, loading: false, needsApiKey: state.needsApiKey, error: state.error);
    } catch (_) {
      // ignore config errors
    }
  }

  Future<void> handleAuthError(AppException e) async {
    if (e.statusCode == 401 || e.code == 'unauthorized') {
      await ref.read(secureStoreProvider).clearSession();
      state = SessionState(oauthConfig: state.oauthConfig);
    } else if (e.statusCode == 428 || e.code == 'api_key_required') {
      state = SessionState(session: state.session, oauthConfig: state.oauthConfig, needsApiKey: true, error: 'api_key_required');
    }
  }

  Future<void> clearSessionLocal() async {
    await ref.read(secureStoreProvider).clearSession();
    state = SessionState(oauthConfig: state.oauthConfig);
  }

  Future<void> login({required String serverUrl, required String userName, required String password}) async {
    state = SessionState(session: state.session, oauthConfig: state.oauthConfig, loading: true);
    try {
      final dio = _dioFor(serverUrl, null);
      final repo = AuthRepository(dio);
      final r = await repo.login(userName: userName, password: password, serverUrl: serverUrl);
      final sess = SessionInfo(serverUrl: r.serverUrl.isNotEmpty ? r.serverUrl : serverUrl, token: r.token, userName: r.userName, mode: r.mode);
      await ref.read(secureStoreProvider).saveSession(serverUrl: sess.serverUrl, token: sess.token, userName: sess.userName, mode: sess.mode);
      if (!r.hasApiKey) {
        state = SessionState(session: sess, oauthConfig: state.oauthConfig, needsApiKey: true, error: 'api_key_required');
      } else {
        state = SessionState(session: sess, oauthConfig: state.oauthConfig);
      }
    } on AppException catch (e) {
      state = SessionState(session: state.session, oauthConfig: state.oauthConfig, error: '${e.code}:${e.message}', needsApiKey: e.code == 'api_key_required' || e.statusCode == 428);
      rethrow;
    } finally {
      if (state.loading) state = SessionState(session: state.session, oauthConfig: state.oauthConfig, error: state.error, needsApiKey: state.needsApiKey);
    }
  }

  Future<void> createAccount({required String serverUrl, required String userName, required String password, String? apiKey}) async {
    state = SessionState(session: state.session, oauthConfig: state.oauthConfig, loading: true);
    try {
      final dio = _dioFor(serverUrl, null);
      final repo = AuthRepository(dio);
      final r = await repo.createAccount(userName: userName, password: password, apiKey: apiKey, serverUrl: serverUrl);
      final sess = SessionInfo(serverUrl: r.serverUrl.isNotEmpty ? r.serverUrl : serverUrl, token: r.token, userName: r.userName, mode: r.mode);
      await ref.read(secureStoreProvider).saveSession(serverUrl: sess.serverUrl, token: sess.token, userName: sess.userName, mode: sess.mode);
      state = SessionState(session: sess, oauthConfig: state.oauthConfig);
    } on AppException catch (e) {
      state = SessionState(session: state.session, oauthConfig: state.oauthConfig, error: '${e.code}:${e.message}');
      rethrow;
    }
  }

  Future<void> setApiKey(String apiKey) async {
    final s = state.session;
    if (s == null) throw Exception('Not logged in');
    final repo = AuthRepository(_dioFor(s.serverUrl, s.token));
    await repo.setApiKey(apiKey);
    state = SessionState(session: s, oauthConfig: state.oauthConfig, needsApiKey: false);
  }

  Future<Map<String, dynamic>> oauthStart({String? serverUrl}) async {
    final s = state.session;
    // On the login screen there is no session yet, so the caller-supplied URL
    // (the field the user typed) must be used instead of the localhost fallback.
    final url = serverUrl ?? s?.serverUrl ?? 'http://localhost:8080';
    final dio = _dioFor(url, s?.token);
    return AuthRepository(dio).oauthStart(serverUrl: url);
  }

  Future<void> oauthFinish(String code, {String? serverUrl}) async {
    final url = serverUrl ?? state.session?.serverUrl ?? 'http://localhost:8080';
    final dio = _dioFor(url, null);
    final r = await AuthRepository(dio).oauthFinish(code);
    final sess = SessionInfo(serverUrl: r.serverUrl.isNotEmpty ? r.serverUrl : url, token: r.token, userName: r.userName, mode: r.mode);
    await ref.read(secureStoreProvider).saveSession(serverUrl: sess.serverUrl, token: sess.token, userName: sess.userName, mode: sess.mode);
    state = SessionState(session: sess, oauthConfig: state.oauthConfig);
  }

  Future<void> logout() async {
    final s = state.session;
    if (s != null) {
      try {
        await AuthRepository(_dioFor(s.serverUrl, s.token)).logout();
      } catch (_) {}
    }
    await ref.read(secureStoreProvider).clearSession();
    state = SessionState(oauthConfig: state.oauthConfig);
  }
}

final sessionProvider = StateNotifierProvider<SessionNotifier, SessionState>((ref) => SessionNotifier(ref));

// Dio bound to current session (or unauthenticated fallback)
final dioProvider = Provider<Dio>((ref) {
  final sess = ref.watch(sessionProvider).session;
  final url = sess?.serverUrl ?? 'http://localhost:8080';
  return ApiClient(serverUrl: url, token: sess?.token).dio;
});

final authRepositoryProvider = Provider<AuthRepository>((ref) => AuthRepository(ref.watch(dioProvider)));
final assetRepositoryProvider = Provider<AssetRepository>((ref) => AssetRepository(ref.watch(dioProvider)));
final albumRepositoryProvider = Provider<AlbumRepository>((ref) => AlbumRepository(ref.watch(dioProvider)));

// ------------------------------------------------------------------
// Settings
// ------------------------------------------------------------------
class SettingsState {
  SettingsState({this.skipVideos = false});
  final bool skipVideos;
  SettingsState copyWith({bool? skipVideos}) => SettingsState(skipVideos: skipVideos ?? this.skipVideos);
}

class SettingsNotifier extends StateNotifier<SettingsState> {
  SettingsNotifier(this.ref) : super(SettingsState()) {
    _restore();
  }
  final Ref ref;
  Future<void> _restore() async {
    try {
      final v = await ref.read(secureStoreProvider).readSkipVideos();
      state = SettingsState(skipVideos: v);
    } catch (_) {}
  }

  Future<void> setSkipVideos(bool v) async {
    state = state.copyWith(skipVideos: v);
    try {
      await ref.read(secureStoreProvider).writeSkipVideos(v);
    } catch (_) {}
  }
}

final settingsProvider = StateNotifierProvider<SettingsNotifier, SettingsState>((ref) => SettingsNotifier(ref));

// ------------------------------------------------------------------
// Stats
// ------------------------------------------------------------------
class StatsState {
  StatsState({this.kept = 0, this.trashed = 0, this.freedBytes = 0});
  final int kept;
  final int trashed;
  final int freedBytes;
  StatsState copyWith({int? kept, int? trashed, int? freedBytes}) => StatsState(kept: kept ?? this.kept, trashed: trashed ?? this.trashed, freedBytes: freedBytes ?? this.freedBytes);
}

class StatsNotifier extends StateNotifier<StatsState> {
  StatsNotifier() : super(StatsState());
  void incKept() => state = state.copyWith(kept: state.kept + 1);
  void incTrashed({int bytes = 0}) => state = state.copyWith(trashed: state.trashed + 1, freedBytes: state.freedBytes + bytes);
  void decKept() => state = state.copyWith(kept: (state.kept - 1).clamp(0, 1 << 30));
  void decTrashed({int bytes = 0}) => state = state.copyWith(trashed: (state.trashed - 1).clamp(0, 1 << 30), freedBytes: (state.freedBytes - bytes).clamp(0, 1 << 60));
  void reset() => state = StatsState();
}

final statsProvider = StateNotifierProvider<StatsNotifier, StatsState>((ref) => StatsNotifier());

// ------------------------------------------------------------------
// Quick albums
// ------------------------------------------------------------------
final quickAlbumsProvider = FutureProvider<List<ImmichAlbum>>((ref) async {
  final repo = ref.watch(albumRepositoryProvider);
  if (ref.watch(sessionProvider).session == null) return [];
  return repo.listAlbums();
});

// ------------------------------------------------------------------
// Review feed
// ------------------------------------------------------------------
enum ReviewActionType { keep, trash }

class ReviewHistoryEntry {
  ReviewHistoryEntry({required this.asset, required this.type, this.freedBytes = 0});
  final ImmichAsset asset;
  final ReviewActionType type;
  final int freedBytes;
}

class ReviewState {
  ReviewState({this.assets = const [], this.loading = false, this.error, this.history = const [], this.reviewedIds = const {}, this.noCandidates = false});
  final List<ImmichAsset> assets;
  final bool loading;
  final String? error;
  final List<ReviewHistoryEntry> history;
  final Set<String> reviewedIds;
  final bool noCandidates;
  static const _unset = Object();
  ReviewState copyWith({List<ImmichAsset>? assets, bool? loading, Object? error = _unset, List<ReviewHistoryEntry>? history, Set<String>? reviewedIds, bool? noCandidates}) =>
      ReviewState(assets: assets ?? this.assets, loading: loading ?? this.loading, error: error == _unset ? this.error : error as String?, history: history ?? this.history, reviewedIds: reviewedIds ?? this.reviewedIds, noCandidates: noCandidates ?? this.noCandidates);
  /// Explicitly clears [error] (copyWith(error: null) also works via sentinel).
  ReviewState clearError() => copyWith(error: null);
}

class ReviewNotifier extends StateNotifier<ReviewState> {
  ReviewNotifier(this.ref) : super(ReviewState());
  final Ref ref;

  bool _fetching = false;
  int _epoch = 0;
  static const int _randomMaxAttempts = 20;

  Set<String> get reviewedIds => state.reviewedIds;

  void _bumpEpoch() => _epoch++;

  Future<void> _handleApiError(Object e) async {
    if (e is AppException) {
      await ref.read(sessionProvider.notifier).handleAuthError(e);
      if (e.statusCode == 401 || e.statusCode == 428) return;
    }
  }

  Future<void> loadInitial() async {
    if (_fetching) return;
    _fetching = true;
    _bumpEpoch();
    final epoch = _epoch;
    state = state.copyWith(loading: true, error: null, noCandidates: false);
    try {
      final sess = ref.read(sessionProvider).session;
      if (sess != null) {
        final stored = await ref.read(secureStoreProvider).readReviewed(sess.serverUrl, sess.userName);
        if (_epoch != epoch) return;
        state = state.copyWith(reviewedIds: stored);
      }
      await _fetchMore(need: 5, epoch: epoch);
    } catch (e) {
      if (_epoch == epoch) state = state.copyWith(error: e.toString());
    } finally {
      if (_epoch == epoch) state = state.copyWith(loading: false);
      _fetching = false;
    }
  }

  Future<void> _fetchMore({int need = 5, int? epoch}) async {
    final myEpoch = epoch ?? _epoch;
    final repo = ref.read(assetRepositoryProvider);
    final skipVideos = ref.read(settingsProvider).skipVideos;
    final existing = state.assets.length;
    if (existing >= need) return;
    try {
      List<ImmichAsset> toAdd = [];
      for (int attempt = 0; attempt < _randomMaxAttempts; attempt++) {
        if (_epoch != myEpoch) return;
        final fetched = await repo.searchRandom(size: 20, imageOnly: skipVideos);
        if (_epoch != myEpoch) return;
        final idsInDeck = state.assets.map((e) => e.id).toSet();
        final alreadyAdded = toAdd.map((e) => e.id).toSet();
        final filtered = fetched.where((a) => !state.reviewedIds.contains(a.id) && !idsInDeck.contains(a.id) && !alreadyAdded.contains(a.id)).toList();
        // skipVideos filtering already done server-side via imageOnly, but double-check
        final usable = skipVideos ? filtered.where((a) => !a.isVideo).toList() : filtered;
        toAdd.addAll(usable);
        if (toAdd.length >= need - existing) {
          toAdd = toAdd.take(need - existing).toList();
          break;
        }
        if (fetched.isEmpty) break;
      }
      if (_epoch != myEpoch) return;
      if (toAdd.isNotEmpty) {
        state = state.copyWith(assets: [...state.assets, ...toAdd], noCandidates: false);
      } else if (state.assets.isEmpty) {
        // No candidates found after retries — distinct from generic error
        state = state.copyWith(noCandidates: true);
      }
    } on AppException catch (e) {
      await _handleApiError(e);
      if (_epoch != myEpoch) return;
      if (e.statusCode == 401 || e.statusCode == 428) return;
      state = state.copyWith(error: e.toString());
    } catch (e) {
      if (_epoch != myEpoch) return;
      state = state.copyWith(error: e.toString());
    }
  }

  int _fileSize(ImmichAsset a) => a.exifInfo?.fileSizeInByte ?? 0;

  Future<void> keepCurrent() async {
    if (state.assets.isEmpty) return;
    final asset = state.assets.first;
    final epoch = _epoch;
    // optimistic update (no persist yet)
    state = state.copyWith(
      assets: state.assets.sublist(1),
      history: [...state.history, ReviewHistoryEntry(asset: asset, type: ReviewActionType.keep)],
    );
    ref.read(statsProvider.notifier).incKept();
    // persist after optimistic — rollback on failure to write? reviewed persist is local, assume success
    try {
      final sess = ref.read(sessionProvider).session;
      final newSet = {...state.reviewedIds, asset.id};
      state = state.copyWith(reviewedIds: newSet);
      if (sess != null) await ref.read(secureStoreProvider).addReviewed(sess.serverUrl, sess.userName, asset.id);
    } catch (_) {}
    if (_epoch != epoch) return;
    unawaited(_fetchMore(need: 5, epoch: epoch));
  }

  Future<void> trashCurrent() async {
    if (state.assets.isEmpty) return;
    final asset = state.assets.first;
    final bytes = _fileSize(asset);
    final snapAssets = List<ImmichAsset>.from(state.assets);
    final snapHistory = List<ReviewHistoryEntry>.from(state.history);
    final snapReviewed = Set<String>.from(state.reviewedIds);
    final snapStats = ref.read(statsProvider);
    final epoch = _epoch;
    // optimistic
    state = state.copyWith(
      assets: state.assets.sublist(1),
      history: [...state.history, ReviewHistoryEntry(asset: asset, type: ReviewActionType.trash, freedBytes: bytes)],
    );
    ref.read(statsProvider.notifier).incTrashed(bytes: bytes);
    try {
      await ref.read(assetRepositoryProvider).trash(asset.id);
      // persist reviewed only after successful trash
      final sess = ref.read(sessionProvider).session;
      final newSet = {...state.reviewedIds, asset.id};
      if (_epoch == epoch) state = state.copyWith(reviewedIds: newSet);
      if (sess != null) await ref.read(secureStoreProvider).addReviewed(sess.serverUrl, sess.userName, asset.id);
    } on AppException catch (e) {
      await _handleApiError(e);
      if (_epoch == epoch) {
        state = state.copyWith(assets: snapAssets, history: snapHistory, reviewedIds: snapReviewed);
        ref.read(statsProvider.notifier).state = snapStats;
      }
      if (e.statusCode == 401 || e.statusCode == 428) return;
      rethrow;
    } catch (e) {
      if (_epoch == epoch) {
        state = state.copyWith(assets: snapAssets, history: snapHistory, reviewedIds: snapReviewed);
        ref.read(statsProvider.notifier).state = snapStats;
      }
      rethrow;
    }
    if (_epoch == epoch) unawaited(_fetchMore(need: 5, epoch: epoch));
  }

  Future<void> undo() async {
    if (state.history.isEmpty) return;
    final last = state.history.last;
    final epoch = _epoch;
    state = state.copyWith(history: state.history.sublist(0, state.history.length - 1), assets: [last.asset, ...state.assets]);
    final newSet = Set<String>.from(state.reviewedIds)..remove(last.asset.id);
    state = state.copyWith(reviewedIds: newSet);
    final sess = ref.read(sessionProvider).session;
    if (sess != null) {
      try { await ref.read(secureStoreProvider).writeReviewed(sess.serverUrl, sess.userName, newSet); } catch (_) {}
    }
    if (last.type == ReviewActionType.keep) {
      ref.read(statsProvider.notifier).decKept();
    } else {
      ref.read(statsProvider.notifier).decTrashed(bytes: last.freedBytes);
      try {
        await ref.read(assetRepositoryProvider).restore(last.asset.id);
        if (_epoch != epoch) return;
      } on AppException catch (e) {
        await _handleApiError(e);
      } catch (_) {}
    }
  }

  Future<void> toggleFavorite(String id) async {
    final idx = state.assets.indexWhere((a) => a.id == id);
    if (idx == -1) return;
    final asset = state.assets[idx];
    final next = !asset.isFavorite;
    asset.isFavorite = next;
    state = state.copyWith(assets: [...state.assets]);
    try {
      await ref.read(assetRepositoryProvider).setFavorite(id, next);
    } on AppException catch (e) {
      await _handleApiError(e);
      asset.isFavorite = !next;
      state = state.copyWith(assets: [...state.assets]);
      if (e.statusCode == 401 || e.statusCode == 428) return;
      rethrow;
    } catch (_) {
      asset.isFavorite = !next;
      state = state.copyWith(assets: [...state.assets]);
      rethrow;
    }
  }

  Future<void> addToAlbum(String albumId, String assetId) async {
    try {
      await ref.read(albumRepositoryProvider).addToAlbum(albumId, assetId);
    } on AppException catch (e) {
      await _handleApiError(e);
      if (e.statusCode == 401 || e.statusCode == 428) return;
      rethrow;
    }
  }

}

void unawaited(Future<void> f) {}

final reviewProvider = StateNotifierProvider<ReviewNotifier, ReviewState>((ref) => ReviewNotifier(ref));
