import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_card_swiper/flutter_card_swiper.dart';
import 'package:video_player/video_player.dart';
import '../../state/providers.dart';
import '../../models/immich_asset.dart';
import 'widgets/swipe_card.dart';
import 'widgets/stats_bar.dart';
import 'widgets/album_chips.dart';

class ReviewScreen extends ConsumerStatefulWidget {
  const ReviewScreen({super.key});
  @override
  ConsumerState<ReviewScreen> createState() => _ReviewScreenState();
}

class _ReviewScreenState extends ConsumerState<ReviewScreen> {
  final CardSwiperController _swiper = CardSwiperController();
  final Map<String, Uint8List> _thumbCache = {};
  final Map<String, VideoPlayerController> _videoControllers = {};
  final Set<String> _pendingVideo = {};

  String? _lastAlbumAssignedId;
  String _prefetchKey = '';

  @override
  void initState() {
    super.initState();
    Future.microtask(() => ref.read(reviewProvider.notifier).loadInitial());
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    // schedule prefetch post-frame so build stays side-effect free
    WidgetsBinding.instance.addPostFrameCallback((_) => _maybePrefetch());
  }

  @override
  void didUpdateWidget(covariant ReviewScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
    WidgetsBinding.instance.addPostFrameCallback((_) => _maybePrefetch());
  }

  void _maybePrefetch() {
    if (!mounted) return;
    final review = ref.read(reviewProvider);
    if (review.assets.isEmpty) {
      _pruneCaches({});
      return;
    }
    final key = review.assets.take(5).map((e) => e.id).join('|');
    if (key == _prefetchKey) return;
    _prefetchKey = key;
    // prefetch thumbnails for visible deck
    for (final a in review.assets.take(5)) {
      _ensureThumb(a);
    }
    // lazily init video only for top card
    final top = review.assets.first;
    if (top.isVideo) {
      _ensureVideo(top);
    }
    _pruneCaches(review.assets.map((e) => e.id).toSet());
  }

  void _pruneCaches(Set<String> liveIds) {
    // evict thumbs not in live deck
    final staleThumbs = _thumbCache.keys.where((k) => !liveIds.contains(k)).toList();
    if (staleThumbs.isNotEmpty) {
      setState(() {
        for (final k in staleThumbs) {
          _thumbCache.remove(k);
        }
      });
    }
    // dispose video controllers not in live deck
    final staleVideos = _videoControllers.keys.where((k) => !liveIds.contains(k)).toList();
    for (final k in staleVideos) {
      final c = _videoControllers.remove(k);
      unawaited(c!.dispose());
    }
    _pendingVideo.removeWhere((k) => !liveIds.contains(k));
  }

  @override
  void dispose() {
    _swiper.dispose();
    for (final c in _videoControllers.values) {
      c.dispose();
    }
    super.dispose();
  }

  Future<void> _ensureThumb(ImmichAsset a) async {
    if (_thumbCache.containsKey(a.id)) return;
    try {
      final bytes = await ref.read(assetRepositoryProvider).fetchThumbnail(a.id, size: 'preview');
      if (mounted) setState(() => _thumbCache[a.id] = bytes);
    } catch (_) {}
  }

  Future<void> _ensureVideo(ImmichAsset a) async {
    if (!a.isVideo || _videoControllers.containsKey(a.id) || _pendingVideo.contains(a.id)) return;
    final sess = ref.read(sessionProvider).session;
    if (sess == null) return;
    final serverUrl = sess.serverUrl;
    final token = sess.token;
    if (serverUrl.isEmpty || token.isEmpty) return;
    _pendingVideo.add(a.id);
    try {
      final uri = Uri.parse('$serverUrl/api/assets/${a.id}/original');
      final controller = VideoPlayerController.networkUrl(uri, httpHeaders: {'Authorization': 'Bearer $token'});
      await controller.initialize();
      await controller.setLooping(true);
      await controller.setVolume(0);
      if (!mounted) {
        await controller.dispose();
        return;
      }
      // verify asset still in deck and not already replaced
      if (_videoControllers.containsKey(a.id)) {
        await controller.dispose();
        return;
      }
      setState(() => _videoControllers[a.id] = controller);
    } catch (_) {
      // silent fallback — remove pending so retry is possible
    } finally {
      _pendingVideo.remove(a.id);
    }
  }

  void _toggleVideo(String assetId) {
    final c = _videoControllers[assetId];
    if (c == null || !c.value.isInitialized) {
      // lazily init on play tap
      final review = ref.read(reviewProvider);
      final match = review.assets.where((e) => e.id == assetId).toList();
      if (match.isNotEmpty) _ensureVideo(match.first);
      return;
    }
    if (c.value.isPlaying) {
      c.pause();
    } else {
      c.play();
    }
    setState(() {});
  }

  void _showExif(ImmichAsset a) {
    final scheme = Theme.of(context).colorScheme;
    final exif = a.exifInfo;
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      showDragHandle: true,
      builder: (c) {
        return SafeArea(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(20, 8, 20, 24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(children: [
                  Container(
                    width: 40,
                    height: 40,
                    decoration: BoxDecoration(
                      color: scheme.primaryContainer,
                      borderRadius: BorderRadius.circular(12),
                    ),
                    child: Icon(Icons.image_rounded, color: scheme.onPrimaryContainer),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                      Text(a.originalFileName,
                          style: Theme.of(context).textTheme.titleSmall?.copyWith(fontWeight: FontWeight.w800),
                          overflow: TextOverflow.ellipsis),
                      const SizedBox(height: 2),
                      Text(_formatDate(a.fileCreatedAt),
                          style: Theme.of(context).textTheme.bodySmall?.copyWith(color: scheme.onSurfaceVariant)),
                    ]),
                  ),
                  Container(
                    padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                    decoration: BoxDecoration(
                      color: a.isVideo ? const Color(0xFF6366F1) : const Color(0xFF10B981),
                      borderRadius: BorderRadius.circular(20),
                    ),
                    child: Text(a.type,
                        style: const TextStyle(color: Colors.white, fontSize: 11, fontWeight: FontWeight.w800)),
                  ),
                ]),
                const SizedBox(height: 20),
                _exifGroup('File', [
                  _row('Name', a.originalFileName),
                  _row('Date', _formatDate(a.fileCreatedAt)),
                  _row('Type', a.type + (a.originalMimeType != null ? ' • ${a.originalMimeType}' : '')),
                  _row('Favorite', a.isFavorite ? 'Yes' : 'No'),
                ]),
                if (exif != null) ...[
                  const SizedBox(height: 16),
                  _exifGroup('Camera', [
                    if (exif.make != null) _row('Make', exif.make!),
                    if (exif.model != null) _row('Model', exif.model!),
                    if (exif.exifImageWidth != null && exif.exifImageHeight != null)
                      _row('Dimensions', '${exif.exifImageWidth} × ${exif.exifImageHeight}'),
                    if (exif.description != null && exif.description!.isNotEmpty) _row('Description', exif.description!),
                  ]),
                  if (exif.city != null || exif.country != null) ...[
                    const SizedBox(height: 16),
                    _exifGroup('Location', [
                      if (exif.city != null) _row('City', exif.city!),
                      if (exif.country != null) _row('Country', exif.country!),
                    ]),
                  ],
                ],
                if (exif?.dateTimeOriginal != null) ...[
                  const SizedBox(height: 16),
                  _exifGroup('Capture', [
                    _row('Original date', _formatDate(exif!.dateTimeOriginal!)),
                  ]),
                ],
                const SizedBox(height: 8),
              ],
            ),
          ),
        );
      },
    );
  }

  Widget _exifGroup(String title, List<Widget> rows) {
    final scheme = Theme.of(context).colorScheme;
    // rows are already filtered by caller; if empty hide group
    if (rows.isEmpty) return const SizedBox.shrink();
    return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
      Text(title,
          style: Theme.of(context).textTheme.labelSmall?.copyWith(
                fontWeight: FontWeight.w800,
                letterSpacing: 0.6,
                color: scheme.primary,
              )),
      const SizedBox(height: 8),
      Container(
        decoration: BoxDecoration(
          color: scheme.surfaceContainerHighest.withValues(alpha: 0.5),
          borderRadius: BorderRadius.circular(14),
          border: Border.all(color: scheme.outlineVariant.withValues(alpha: 0.4)),
        ),
        child: Column(children: rows),
      ),
    ]);
  }

  Widget _row(String k, String v) {
    if (v.isEmpty) return const SizedBox.shrink();
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
      child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
        SizedBox(
          width: 110,
          child: Text(k,
              style: Theme.of(context).textTheme.bodySmall?.copyWith(
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                    fontWeight: FontWeight.w600,
                  )),
        ),
        Expanded(
          child: SelectableText(v,
              style: Theme.of(context).textTheme.bodySmall?.copyWith(fontWeight: FontWeight.w600)),
        ),
      ]),
    );
  }

  String _formatDate(String raw) {
    if (raw.isEmpty) return '—';
    try {
      final d = DateTime.parse(raw);
      const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
      return '${months[d.month - 1]} ${d.day}, ${d.year}';
    } catch (_) {
      return raw.length > 10 ? raw.substring(0, 10) : raw;
    }
  }

  @override
  Widget build(BuildContext context) {
    final review = ref.watch(reviewProvider);
    final stats = ref.watch(statsProvider);
    final albumsAsync = ref.watch(quickAlbumsProvider);

    // FIX: deck loads asynchronously AFTER mount, so didChangeDependencies alone
    // never re-ran the prefetch and no thumbnail was ever requested. React to
    // every deck change instead (post-frame keeps build side-effect free).
    ref.listen(reviewProvider, (_, _) {
      WidgetsBinding.instance.addPostFrameCallback((_) => _maybePrefetch());
    });

    final scheme = Theme.of(context).colorScheme;
    final isDark = Theme.of(context).brightness == Brightness.dark;

    // Shared app bar
    PreferredSizeWidget appBar({bool showLogout = true}) => AppBar(
          title: Row(children: [
            Container(
              width: 36,
              height: 36,
              decoration: BoxDecoration(
                gradient: const LinearGradient(colors: [Color(0xFF6366F1), Color(0xFF8B5CF6)]),
                borderRadius: BorderRadius.circular(10),
              ),
              child: const Icon(Icons.style_rounded, size: 18, color: Colors.white),
            ),
            const SizedBox(width: 10),
            Text('Review',
                style: Theme.of(context).textTheme.titleMedium?.copyWith(fontWeight: FontWeight.w800)),
            const SizedBox(width: 8),
            if (review.assets.isNotEmpty)
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                decoration: BoxDecoration(
                  color: isDark ? Colors.white.withValues(alpha: 0.08) : scheme.primaryContainer,
                  borderRadius: BorderRadius.circular(20),
                  border: Border.all(color: scheme.outlineVariant.withValues(alpha: 0.3)),
                ),
                child: Text('${review.assets.length} left',
                    style: Theme.of(context).textTheme.labelSmall?.copyWith(
                          fontWeight: FontWeight.w800,
                          fontSize: 11,
                          color: isDark ? Colors.white70 : scheme.onPrimaryContainer,
                        )),
              ),
          ]),
          actions: [
            if (showLogout)
              IconButton(
                onPressed: () => ref.read(sessionProvider.notifier).logout(),
                icon: const Icon(Icons.logout_rounded, size: 20),
                tooltip: 'Sign out',
                style: IconButton.styleFrom(minimumSize: const Size(48, 48)),
              ),
          ],
        );

    // Loading
    if (review.loading && review.assets.isEmpty) {
      return Scaffold(
        appBar: appBar(showLogout: true),
        body: SafeArea(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(children: [
              StatsBar(kept: stats.kept, trashed: stats.trashed, freedBytes: stats.freedBytes),
              const SizedBox(height: 16),
              Expanded(
                child: Container(
                  decoration: BoxDecoration(
                    color: isDark ? Colors.white.withValues(alpha: 0.04) : Colors.black.withValues(alpha: 0.04),
                    borderRadius: BorderRadius.circular(24),
                    border: Border.all(color: scheme.outlineVariant.withValues(alpha: 0.2)),
                  ),
                  child: Center(
                    child: Column(mainAxisSize: MainAxisSize.min, children: [
                      SizedBox(
                        width: 36,
                        height: 36,
                        child: CircularProgressIndicator(strokeWidth: 3, color: scheme.primary),
                      ),
                      const SizedBox(height: 16),
                      Text('Loading photos…',
                          style:
                              Theme.of(context).textTheme.titleSmall?.copyWith(fontWeight: FontWeight.w700)),
                      const SizedBox(height: 6),
                      Text('Fetching your library',
                          style: Theme.of(context)
                              .textTheme
                              .bodySmall
                              ?.copyWith(color: scheme.onSurfaceVariant)),
                    ]),
                  ),
                ),
              ),
            ]),
          ),
        ),
      );
    }

    // Error (no asset but error present) or empty
    if (review.assets.isEmpty) {
      final isError = review.error != null;
      return Scaffold(
        appBar: appBar(),
        body: SafeArea(
          child: Padding(
            padding: EdgeInsets.fromLTRB(16, 8, 16, MediaQuery.of(context).padding.bottom + 12),
            child: Column(children: [
              StatsBar(kept: stats.kept, trashed: stats.trashed, freedBytes: stats.freedBytes),
              const SizedBox(height: 16),
              Expanded(
                child: Center(
                  child: ConstrainedBox(
                    constraints: const BoxConstraints(maxWidth: 420),
                    child: Column(mainAxisSize: MainAxisSize.min, children: [
                      Container(
                        width: 84,
                        height: 84,
                        decoration: BoxDecoration(
                          color: isError
                              ? scheme.errorContainer.withValues(alpha: isDark ? 0.4 : 1)
                              : scheme.primaryContainer.withValues(alpha: isDark ? 0.4 : 1),
                          shape: BoxShape.circle,
                        ),
                        child: Icon(
                          isError ? Icons.cloud_off_rounded : Icons.check_circle_rounded,
                          size: 42,
                          color: isError ? scheme.onErrorContainer : scheme.primary,
                        ),
                      ),
                      const SizedBox(height: 20),
                      Text(
                        isError ? 'Couldn’t load photos' : 'All caught up',
                        style: Theme.of(context).textTheme.titleLarge?.copyWith(fontWeight: FontWeight.w800),
                        textAlign: TextAlign.center,
                      ),
                      const SizedBox(height: 8),
                      Text(
                        isError
                            ? (review.error ?? 'Something went wrong. Check your server connection and try again.')
                            : 'Nothing left to review. Pull to reload or check back later — your library is tidy.',
                        style: Theme.of(context)
                            .textTheme
                            .bodyMedium
                            ?.copyWith(color: scheme.onSurfaceVariant, height: 1.5),
                        textAlign: TextAlign.center,
                      ),
                      const SizedBox(height: 20),
                      FilledButton.icon(
                        onPressed: () => ref.read(reviewProvider.notifier).loadInitial(),
                        icon: Icon(isError ? Icons.refresh_rounded : Icons.replay_rounded, size: 18),
                        label: Text(isError ? 'Try again' : 'Reload'),
                        style: FilledButton.styleFrom(minimumSize: const Size(48, 48)),
                      ),
                      if (isError && review.error != null)
                        Padding(
                          padding: const EdgeInsets.only(top: 12),
                          child: Container(
                            width: double.infinity,
                            padding: const EdgeInsets.all(12),
                            decoration: BoxDecoration(
                              color: scheme.errorContainer.withValues(alpha: isDark ? 0.3 : 0.6),
                              borderRadius: BorderRadius.circular(12),
                              border: Border.all(color: scheme.error.withValues(alpha: 0.2)),
                            ),
                            child: Text(review.error!,
                                style: Theme.of(context)
                                    .textTheme
                                    .bodySmall
                                    ?.copyWith(color: scheme.onErrorContainer)),
                          ),
                        ),
                      const SizedBox(height: 16),
                      // keep stats visible as second instance? already at top; show extra hint
                      if (!isError)
                        Container(
                          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
                          decoration: BoxDecoration(
                            color: isDark ? Colors.white.withValues(alpha: 0.06) : Colors.white,
                            borderRadius: BorderRadius.circular(12),
                            border: Border.all(color: scheme.outlineVariant.withValues(alpha: 0.2)),
                          ),
                          child: Row(mainAxisSize: MainAxisSize.min, children: [
                            Icon(Icons.insights_rounded, size: 16, color: scheme.primary),
                            const SizedBox(width: 8),
                            Text('Kept ${stats.kept} • Trashed ${stats.trashed}',
                                style: Theme.of(context).textTheme.labelMedium?.copyWith(
                                      fontWeight: FontWeight.w700,
                                      color: scheme.onSurfaceVariant,
                                    )),
                          ]),
                        ),
                    ]),
                  ),
                ),
              ),
            ]),
          ),
        ),
      );
    }

    // Main deck
    return Scaffold(
      appBar: appBar(),
      body: SafeArea(
        bottom: true,
        child: Padding(
          padding: EdgeInsets.fromLTRB(16, 8, 16, MediaQuery.of(context).padding.bottom + 8),
          child: LayoutBuilder(builder: (context, constraints) {
            // Ensure no overflow at 320dp: card width is constrained.
            final maxDeckWidth = constraints.maxWidth > 520 ? 520.0 : constraints.maxWidth;
            return Column(children: [
              StatsBar(kept: stats.kept, trashed: stats.trashed, freedBytes: stats.freedBytes),
              const SizedBox(height: 12),
              Expanded(
                child: Center(
                  child: ConstrainedBox(
                    constraints: BoxConstraints(maxWidth: maxDeckWidth, maxHeight: constraints.maxHeight * 0.72),
                    child: CardSwiper(
                      controller: _swiper,
                      cardsCount: review.assets.length,
                      numberOfCardsDisplayed: review.assets.length >= 2 ? 2 : review.assets.length,
                      backCardOffset: const Offset(0, 14),
                      padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 8),
                      maxAngle: 28,
                      threshold: 55,
                      scale: 0.94,
                      isLoop: false,
                      allowedSwipeDirection: const AllowedSwipeDirection.only(left: true, right: true),
                      onSwipe: (oldIdx, currentIdx, direction) async {
                        HapticFeedback.lightImpact();
                        // FIX 3: resolve swiped asset BEFORE await that mutates deck
                        final swiped = review.assets[oldIdx];
                        if (direction == CardSwiperDirection.left) {
                          await ref.read(reviewProvider.notifier).trashCurrent();
                        } else if (direction == CardSwiperDirection.right) {
                          await ref.read(reviewProvider.notifier).keepCurrent();
                        } else {
                          return false;
                        }
                        final vc = _videoControllers.remove(swiped.id);
                        if (vc != null) unawaited(vc.dispose());
                        _pendingVideo.remove(swiped.id);
                        // prune thumb cache to live deck
                        WidgetsBinding.instance.addPostFrameCallback((_) {
                          if (mounted) {
                            final live = ref.read(reviewProvider).assets.map((e) => e.id).toSet();
                            _pruneCaches(live);
                            _maybePrefetch();
                          }
                        });
                        return true;
                      },
                      onUndo: (prevIdx, currIdx, dir) => true,
                      cardBuilder: (context, index, hPercent, vPercent) {
                        final a = review.assets[index];
                        // subtle rotation tied to swipe progress for peek card
                        final isTop = index == 0;
                        final rotate = isTop ? 0.0 : (hPercent * 0.04);
                        return Transform.rotate(
                          angle: rotate,
                          child: Stack(children: [
                            GestureDetector(
                              onTap: a.isVideo ? () => _toggleVideo(a.id) : null,
                              child: SwipeCard(
                                asset: a,
                                imageBytes: _thumbCache[a.id],
                                videoController: _videoControllers[a.id],
                                onFavoriteToggle: () => ref.read(reviewProvider.notifier).toggleFavorite(a.id),
                                onInfo: () => _showExif(a),
                              ),
                            ),
                            // swipe hint overlay (peeks as you drag top card: show on top card via builder progress)
                            if (isTop)
                              Positioned.fill(
                                child: IgnorePointer(
                                  child: AnimatedOpacity(
                                    opacity: (hPercent.abs() / 100).clamp(0, 1) * 0.9,
                                    duration: const Duration(milliseconds: 60),
                                    child: Container(
                                      decoration: BoxDecoration(
                                        borderRadius: BorderRadius.circular(24),
                                        border: Border.all(
                                          color: hPercent > 0
                                              ? const Color(0xFF10B981).withValues(alpha: 0.8)
                                              : const Color(0xFFEF4444).withValues(alpha: 0.8),
                                          width: 3,
                                        ),
                                        color: (hPercent > 0 ? const Color(0xFF10B981) : const Color(0xFFEF4444))
                                            .withValues(alpha: 0.08),
                                      ),
                                      alignment: hPercent > 0 ? Alignment.centerLeft : Alignment.centerRight,
                                      padding: const EdgeInsets.symmetric(horizontal: 20),
                                      child: Container(
                                        padding:
                                            const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
                                        decoration: BoxDecoration(
                                          color: hPercent > 0 ? const Color(0xFF10B981) : const Color(0xFFEF4444),
                                          borderRadius: BorderRadius.circular(12),
                                          boxShadow: [
                                            BoxShadow(
                                                color: Colors.black.withValues(alpha: 0.2),
                                                blurRadius: 10,
                                                offset: const Offset(0, 4))
                                          ],
                                        ),
                                        child: Row(mainAxisSize: MainAxisSize.min, children: [
                                          Icon(
                                              hPercent > 0 ? Icons.check_rounded : Icons.delete_rounded,
                                              color: Colors.white,
                                              size: 18),
                                          const SizedBox(width: 6),
                                          Text(hPercent > 0 ? 'KEEP' : 'TRASH',
                                              style: const TextStyle(
                                                  color: Colors.white,
                                                  fontWeight: FontWeight.w900,
                                                  fontSize: 13,
                                                  letterSpacing: 0.6)),
                                        ]),
                                      ),
                                    ),
                                  ),
                                ),
                              ),
                          ]),
                        );
                      },
                    ),
                  ),
                ),
              ),
              const SizedBox(height: 10),
              // Hint row
              Row(mainAxisAlignment: MainAxisAlignment.center, children: [
                _hintPill(context, icon: Icons.swipe_left_rounded, label: 'Trash', color: const Color(0xFFEF4444), isDark: isDark),
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 8),
                  child: Icon(Icons.swap_horiz_rounded, size: 16, color: scheme.onSurfaceVariant.withValues(alpha: 0.6)),
                ),
                _hintPill(context, icon: Icons.swipe_right_rounded, label: 'Keep', color: const Color(0xFF10B981), isDark: isDark),
              ]),
              const SizedBox(height: 12),
              // Quick album chips
              albumsAsync.when(
                data: (albums) => albums.isEmpty
                    ? const SizedBox.shrink()
                    : AlbumChips(
                        albums: albums,
                        onSelect: (al) async {
                          if (review.assets.isEmpty) return;
                          final cur = review.assets.first;
                          try {
                            await ref.read(reviewProvider.notifier).addToAlbum(al.id, cur.id);
                            if (!context.mounted) return;
                            HapticFeedback.selectionClick();
                            setState(() => _lastAlbumAssignedId = al.id);
                            ScaffoldMessenger.of(context).showSnackBar(
                              SnackBar(
                                content: Text('Added to ${al.albumName}'),
                                behavior: SnackBarBehavior.floating,
                                duration: const Duration(milliseconds: 1400),
                              ),
                            );
                            Future.delayed(const Duration(milliseconds: 1400), () {
                              if (mounted && _lastAlbumAssignedId == al.id) {
                                setState(() => _lastAlbumAssignedId = null);
                              }
                            });
                          } catch (e) {
                            if (!context.mounted) return;
                            ScaffoldMessenger.of(context).showSnackBar(
                              SnackBar(content: Text('Album add failed: $e'), behavior: SnackBarBehavior.floating),
                            );
                          }
                        },
                      ),
                loading: () => const SizedBox.shrink(),
                error: (_, _) => const SizedBox.shrink(),
              ),
              const SizedBox(height: 12),
              // Bottom action bar — touch-first: Undo / Favorite / Info. No desktop Keep/Delete.
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 10),
                decoration: BoxDecoration(
                  color: isDark ? Colors.white.withValues(alpha: 0.06) : Colors.white,
                  borderRadius: BorderRadius.circular(20),
                  border: Border.all(color: scheme.outlineVariant.withValues(alpha: isDark ? 0.15 : 0.4)),
                  boxShadow: isDark
                      ? []
                      : [BoxShadow(color: Colors.black.withValues(alpha: 0.06), blurRadius: 16, offset: const Offset(0, 8))],
                ),
                child: Row(children: [
                  Expanded(
                    child: _actionBtn(
                      context,
                      icon: Icons.undo_rounded,
                      label: 'Undo',
                      enabled: review.history.isNotEmpty,
                      onTap: () async {
                        if (review.history.isEmpty) return;
                        HapticFeedback.selectionClick();
                        await ref.read(reviewProvider.notifier).undo();
                        HapticFeedback.lightImpact();
                      },
                      isDark: isDark,
                      scheme: scheme,
                    ),
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: _favoriteAction(context, review.assets.first, isDark, scheme),
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: _actionBtn(
                      context,
                      icon: Icons.info_outline_rounded,
                      label: 'Info',
                      enabled: true,
                      onTap: () => _showExif(review.assets.first),
                      isDark: isDark,
                      scheme: scheme,
                      filled: false,
                    ),
                  ),
                ]),
              ),
              if (review.error != null)
                Padding(
                  padding: const EdgeInsets.only(top: 8),
                  child: Container(
                    width: double.infinity,
                    padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
                    decoration: BoxDecoration(
                      color: scheme.errorContainer.withValues(alpha: isDark ? 0.3 : 0.7),
                      borderRadius: BorderRadius.circular(12),
                    ),
                    child: Row(children: [
                      Icon(Icons.error_outline_rounded, size: 16, color: scheme.onErrorContainer),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Text(review.error!,
                            style: Theme.of(context)
                                .textTheme
                                .bodySmall
                                ?.copyWith(color: scheme.onErrorContainer, fontWeight: FontWeight.w600),
                            maxLines: 2,
                            overflow: TextOverflow.ellipsis),
                      ),
                    ]),
                  ),
                ),
            ]);
          }),
        ),
      ),
    );
  }

  Widget _hintPill(BuildContext context,
      {required IconData icon, required String label, required Color color, required bool isDark}) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
      decoration: BoxDecoration(
        color: color.withValues(alpha: isDark ? 0.15 : 0.1),
        borderRadius: BorderRadius.circular(20),
        border: Border.all(color: color.withValues(alpha: 0.25)),
      ),
      child: Row(mainAxisSize: MainAxisSize.min, children: [
        Icon(icon, size: 14, color: color),
        const SizedBox(width: 6),
        Text(label,
            style: Theme.of(context).textTheme.labelSmall?.copyWith(
                  fontWeight: FontWeight.w800,
                  color: color,
                  letterSpacing: 0.3,
                )),
      ]),
    );
  }

  Widget _actionBtn(
    BuildContext context, {
    required IconData icon,
    required String label,
    required bool enabled,
    required VoidCallback onTap,
    required bool isDark,
    required ColorScheme scheme,
    bool filled = false,
  }) {
    final bg = filled || !enabled
        ? (enabled ? scheme.primary : scheme.surfaceContainerHighest)
        : (isDark ? Colors.white.withValues(alpha: 0.08) : scheme.surfaceContainerHighest);
    final fg = enabled
        ? (filled ? scheme.onPrimary : scheme.onSurface)
        : scheme.onSurfaceVariant.withValues(alpha: 0.5);
    return SizedBox(
      height: 48,
      child: Material(
        color: bg,
        borderRadius: BorderRadius.circular(14),
        child: InkWell(
          onTap: enabled ? onTap : null,
          borderRadius: BorderRadius.circular(14),
          child: Container(
            alignment: Alignment.center,
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(14),
              border: Border.all(
                  color: filled
                      ? Colors.transparent
                      : scheme.outlineVariant.withValues(alpha: isDark ? 0.15 : 0.5)),
            ),
            child: Row(mainAxisAlignment: MainAxisAlignment.center, children: [
              Icon(icon, size: 18, color: fg),
              const SizedBox(width: 8),
              Text(label,
                  style: Theme.of(context).textTheme.labelLarge?.copyWith(
                        fontWeight: FontWeight.w700,
                        color: fg,
                        fontSize: 13,
                      )),
            ]),
          ),
        ),
      ),
    );
  }

  Widget _favoriteAction(BuildContext context, ImmichAsset asset, bool isDark, ColorScheme scheme) {
    final isFav = asset.isFavorite;
    return SizedBox(
      height: 48,
      child: Material(
        color: isFav ? const Color(0xFFFF3B5C) : (isDark ? Colors.white.withValues(alpha: 0.08) : scheme.surfaceContainerHighest),
        borderRadius: BorderRadius.circular(14),
        child: InkWell(
          onTap: () async {
            HapticFeedback.selectionClick();
            try {
              await ref.read(reviewProvider.notifier).toggleFavorite(asset.id);
              HapticFeedback.lightImpact();
            } catch (e) {
              if (!context.mounted) return;
              ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Favorite failed: $e')));
            }
          },
          borderRadius: BorderRadius.circular(14),
          child: Container(
            alignment: Alignment.center,
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(14),
              border: Border.all(
                  color: isFav ? Colors.transparent : scheme.outlineVariant.withValues(alpha: isDark ? 0.15 : 0.5)),
            ),
            child: Row(mainAxisAlignment: MainAxisAlignment.center, children: [
              Icon(isFav ? Icons.favorite_rounded : Icons.favorite_border_rounded,
                  size: 18, color: isFav ? Colors.white : scheme.onSurface),
              const SizedBox(width: 8),
              Text(isFav ? 'Saved' : 'Favorite',
                  style: Theme.of(context).textTheme.labelLarge?.copyWith(
                        fontWeight: FontWeight.w700,
                        color: isFav ? Colors.white : scheme.onSurface,
                        fontSize: 13,
                      )),
            ]),
          ),
        ),
      ),
    );
  }
}
