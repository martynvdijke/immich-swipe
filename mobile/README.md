# Immich Swipe — Flutter Android App (`mobile/`)

Native Android client for the immich-swipe Go backend. Plain Material baseline — a designer will elevate visuals afterwards.

## Run

```
cd mobile
flutter pub get
flutter analyze
flutter test
# Run on device/emulator (needs Android SDK):
flutter run
```

The Server URL field ships **empty** with an example hint (`https://immich.example.com`) — there is no hardcoded default, so a release build never points at a developer machine. When running against a local backend on the Android emulator type `http://10.0.2.2:8080` (emulator loopback to the host); for a physical device use `http://<host-ip>:8080`.

## Local Android build (verified)

Verified on this host: `app-release.apk` (~53 MB) builds with

```
export JAVA_HOME=/usr/lib/jvm/java-17-openjdk   # AGP 9.1.0 + sourceCompatibility 17 need JDK 17; the system default `java` is 11
export ANDROID_HOME=/root/Android/Sdk ANDROID_SDK_ROOT=/root/Android/Sdk
export PATH="$JAVA_HOME/bin:$PATH"
flutter config --android-sdk /root/Android/Sdk
cd mobile && flutter build apk --release
```

Output: `build/app/outputs/flutter-apk/app-release.apk` — `minSdk 24`, `targetSdk`/`compileSdk 36`, `versionName` from `pubspec.yaml`, ABIs `arm64-v8a`, `armeabi-v7a`, `x86_64`.

Required SDK packages: `platforms;android-36`, `build-tools;36.0.0`, `ndk;28.2.13676358`, `cmake;3.22.1`. The NDK/CMake are not optional: `path_provider_android` → `jni_flutter` → `jni` compiles C through CMake and produces `libdartjni.so`. `.github/workflows/flutter-android.yaml` and the `android` job in `publish-ghcr.yml` install exactly these packages explicitly.

Gotcha: if a build is interrupted mid-CMake it can leave stale native state in the pub cache, and the next build fails with a bare `cmake ... finished with non-zero exit value 1`. Fix:

```
rm -rf ~/.pub-cache/hosted/pub.dev/jni-1.1.0/android/.cxx
flutter clean
```

## Backend API Contract

All swipe-session calls are relative to `<serverUrl>/api` with header `Authorization: Bearer <swipe-session-token>`. The Go proxy strips that header and injects Immich credentials.

- `POST /api/auth/login` `{userName,password,serverUrl?}` -> `{token,userName,serverUrl,mode:"apiKey"|"accessToken",hasApiKey}`. Creation variant adds `{create:true, apiKey?}`. Errors: 400 `weak_password`/`account_exists`, 401 `unknown_user`/`password_not_set`/`invalid_password`/`invalid_api_key`, 428 `api_key_required`.
- `POST /api/auth/account/apikey` `{apiKey}` (apiKey mode only)
- `GET /api/auth/config` -> `{oauthEnabled,oauthButtonText}`
- `POST /api/auth/oauth/start` `{serverUrl?}` -> `{url,state}` ; `POST /api/auth/oauth/finish` `{code}` -> session.
- `POST /api/auth/logout`
- `GET /api/users/me` (connection check)
- `POST /api/search/metadata` `{take,size,skip,order,assetType,type}` -> `{assets:{total,count,items,nextPage}}`
- `POST /api/search/random` `{size,type?}` -> `ImmichAsset[]`
- `DELETE /api/assets` `{ids:[id], force:false}` (trash), `POST /api/trash/restore/assets` `{ids:[id]}` (undo)
- `PUT /api/assets/{id}` `{isFavorite:bool}`
- `GET /api/albums` and `GET /api/albums?isShared=true` (dedupe by id), `PUT /api/albums/{albumId}/assets` `{ids:[id]}`
- `GET /api/assets/{id}/thumbnail?size=thumbnail|preview` and `GET /api/assets/{id}/original` — **must be loaded with dio as bytes + `Authorization` header and rendered via `Image.memory` / video file source; do NOT use plain `Image.network`.**

## Project Map

### Core

| Class / Symbol | File | Notes |
|---|---|---|
| `ApiClient` + `AppException` | `lib/core/api_client.dart` | Dio with `baseUrl=<server>/api` + auth interceptor; `mapError` normalizes `DioException` |
| `SecureStore` | `lib/core/secure_store.dart` | `flutter_secure_storage`: `serverUrl, sessionToken, userName, mode` + `reviewed_<server>_<user>` id set |
| `formatBytes` | `lib/core/format.dart` | bytes -> KB/MB/GB for stats bar |

### Models

| Class | File |
|---|---|
| `ImmichAsset`, `ExifInfo`, `MetadataSearchResponse` | `lib/models/immich_asset.dart` |
| `ImmichAlbum`, `ImmichPerson` | `lib/models/immich_album.dart` |
| `SessionInfo`, `LoginResult`, `AuthConfig` | `lib/models/session.dart` |

### Data (typed repos, dio error -> `AppException`)

| Class | File |
|---|---|
| `AuthRepository` | `lib/data/auth_repository.dart` |
| `AssetRepository` | `lib/data/asset_repository.dart` |
| `AlbumRepository` | `lib/data/album_repository.dart` |

### State (Riverpod)

| Provider | File | Description |
|---|---|---|
| `secureStoreProvider` | `lib/state/providers.dart` | `SecureStore` singleton |
| `sessionProvider` (`SessionNotifier` / `SessionState`) | `lib/state/providers.dart` | login / create / setApiKey / oauthStart+Finish / logout / restore-on-start; exposes `oauthConfig` |
| `dioProvider` | `lib/state/providers.dart` | Dio bound to current session |
| `authRepositoryProvider`, `assetRepositoryProvider`, `albumRepositoryProvider` | `lib/state/providers.dart` | Repos using `dioProvider` |
| `settingsProvider` (`SettingsNotifier`) | `lib/state/providers.dart` | `skipVideos`, `scope` |
| `statsProvider` (`StatsNotifier` / `StatsState`) | `lib/state/providers.dart` | `kept`, `trashed`, `freedBytes` |
| `quickAlbumsProvider` (FutureProvider) | `lib/state/providers.dart` | Deduplicated albums for chip bar |
| `reviewProvider` (`ReviewNotifier` / `ReviewState`) | `lib/state/providers.dart` | Deck of unreviewed assets, prefetch next 5, undo stack with restore call, keep/trash/favorite, reviewed-id persistence |

### Features / Widgets

| Widget | File | Constructor / Params | Presentation? |
|---|---|---|---|
| `LoginScreen` | `lib/features/auth/login_screen.dart` | `const LoginScreen()` — reads `sessionProvider` | No (stateful, handles auth) |
| `ReviewScreen` | `lib/features/review/review_screen.dart` | `const ReviewScreen()` — `ConsumerStatefulWidget` | No (holds `CardSwiperController`, preloads, video controllers) |
| `SwipeCard` | `lib/features/review/widgets/swipe_card.dart` | `const SwipeCard({required asset, imageBytes, videoController, videoLoading = false, onFavoriteToggle, onInfo})` | **Yes** — pure card, no providers |

### Video playback

The player is created **only from an explicit tap** on a video card (one tap initialises and starts
playback; while it initialises the card shows a spinner, and a failure surfaces as a snack bar).
`ReviewScreen` deliberately does **not** create video controllers while prefetching: creating one
pulls the entire original file, so prefetching would download a full-size video merely because a
video card reached the top of the deck. Thumbnails are still prefetched for the visible deck.

Note (measured on the emulator rig against the mock backend): once a player exists, ExoPlayer
Media3 re-opens the same `.../original` URL several times (six re-opens ~1 s apart, plus roughly one
per loop during playback) even though each response delivers the complete body and the client never
aborts. That was reproduced with a faststart and a non-faststart file, with buffered and streamed
responses, and with/without `Content-Range`, and is not caused by this app's request pattern
(ExoPlayer sends no `Range` header and there is exactly one player instance).
| `StatsBar` | `lib/features/review/widgets/stats_bar.dart` | `const StatsBar({required kept, trashed, freedBytes})` | **Yes** |
| `AlbumChips` | `lib/features/review/widgets/album_chips.dart` | `const AlbumChips({required albums, required onSelect})` | **Yes** |
| `MyApp` | `lib/main.dart` | `const MyApp()` — switches `LoginScreen` vs `ReviewScreen` by `sessionProvider` | No |

**Pure presentation widgets** (designer can restyle freely without touching logic): `SwipeCard`, `StatsBar`, `AlbumChips`.

## Android Config

- `applicationId` / package `xyz.vandijke.immich_swipe`
- `android:label` = `Immich Swipe`
- `INTERNET` permission in `AndroidManifest.xml`

## SSO (in-app browser)

`GET /api/auth/config` drives whether the SSO button is shown. Availability is
checked for the URL currently in the Server URL field (on focus loss) as well as
after a session restore, so a fresh install can use SSO too.

Tapping the button calls `POST /api/auth/oauth/start` and runs the returned
Immich authorize URL inside `OAuthWebViewScreen` (`webview_flutter`), an in-app
browser. After the user authenticates, the IdP redirects to
`/api/auth/oauth/callback`, which the Go backend answers with
`302 <swipe>/login?oauthCode=<code>`; the screen intercepts that navigation
(the SPA route is never loaded), pops the one-time code, and the login screen
exchanges it via `POST /api/auth/oauth/finish`.

Fallbacks, kept deliberately:

- The WebView's app bar has **Copy SSO URL**; the manual *One-time OAuth code*
  field plus **Finish SSO login** button still work for providers that refuse to
  render in an embedded browser.
- If the start call returns `oauth_not_enabled`, the error is surfaced inline
  instead of opening the browser.

Helpers live in `lib/core/oauth.dart` (`oauthCodeFromUrl`, `oauthErrorFromUrl`)
and are unit-tested without a platform WebView.

## Tests

See `test/` — pure logic + widget tests with faked repositories / overridden providers, no network or Android SDK needed.
