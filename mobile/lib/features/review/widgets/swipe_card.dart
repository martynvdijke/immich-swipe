import 'dart:typed_data';
import 'package:flutter/material.dart';
import 'package:video_player/video_player.dart';
import '../../../models/immich_asset.dart';

/// Pure presentation card for a single asset.
/// Loads bytes via Image.memory (never Image.network) — caller supplies bytes.
class SwipeCard extends StatelessWidget {
  const SwipeCard({super.key, required this.asset, this.imageBytes, this.videoController, this.onFavoriteToggle, this.onInfo});
  final ImmichAsset asset;
  final Uint8List? imageBytes;
  final VideoPlayerController? videoController;
  final VoidCallback? onFavoriteToggle;
  final VoidCallback? onInfo;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final isDark = Theme.of(context).brightness == Brightness.dark;

    Widget media;
    if (asset.isVideo && videoController != null && videoController!.value.isInitialized) {
      media = ClipRRect(
        borderRadius: BorderRadius.circular(24),
        child: Stack(fit: StackFit.expand, children: [
          FittedBox(
            fit: BoxFit.cover,
            child: SizedBox(
              width: videoController!.value.size.width,
              height: videoController!.value.size.height,
              child: VideoPlayer(videoController!),
            ),
          ),
          // subtle play affordance when paused
          if (!videoController!.value.isPlaying)
            Center(
              child: Container(
                width: 56,
                height: 56,
                decoration: BoxDecoration(
                  color: Colors.black.withValues(alpha: 0.45),
                  shape: BoxShape.circle,
                  border: Border.all(color: Colors.white.withValues(alpha: 0.8), width: 1.5),
                ),
                child: const Icon(Icons.play_arrow_rounded, color: Colors.white, size: 32),
              ),
            ),
        ]),
      );
    } else if (imageBytes != null) {
      media = ClipRRect(
        borderRadius: BorderRadius.circular(24),
        child: Image.memory(imageBytes!, fit: BoxFit.cover, width: double.infinity, height: double.infinity),
      );
    } else {
      // shimmer-like placeholder
      media = ClipRRect(
        borderRadius: BorderRadius.circular(24),
        child: Container(
          color: isDark ? const Color(0xFF1E1E32) : const Color(0xFFE8E8F0),
          child: Center(
            child: Column(mainAxisSize: MainAxisSize.min, children: [
              SizedBox(
                width: 28,
                height: 28,
                child: CircularProgressIndicator(strokeWidth: 2.5, color: scheme.primary),
              ),
              const SizedBox(height: 12),
              Text('Loading…',
                  style: Theme.of(context).textTheme.labelMedium?.copyWith(
                        color: scheme.onSurfaceVariant,
                        fontWeight: FontWeight.w600,
                      )),
            ]),
          ),
        ),
      );
    }

    final dateLabel = _formatDate(asset.fileCreatedAt);

    return Container(
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(24),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withValues(alpha: isDark ? 0.45 : 0.18),
            blurRadius: 24,
            offset: const Offset(0, 12),
          ),
        ],
      ),
      child: Card(
        margin: EdgeInsets.zero,
        clipBehavior: Clip.antiAlias,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(24)),
        child: Stack(fit: StackFit.expand, children: [
          media,

          // video badge
          if (asset.isVideo)
            Positioned(
              top: 12,
              left: 12,
              child: Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                decoration: BoxDecoration(
                  color: Colors.black.withValues(alpha: 0.6),
                  borderRadius: BorderRadius.circular(20),
                  border: Border.all(color: Colors.white.withValues(alpha: 0.2)),
                ),
                child: Row(mainAxisSize: MainAxisSize.min, children: [
                  const Icon(Icons.videocam_rounded, size: 14, color: Colors.white),
                  const SizedBox(width: 4),
                  Text(asset.duration ?? 'Video',
                      style: const TextStyle(color: Colors.white, fontSize: 11, fontWeight: FontWeight.w700)),
                ]),
              ),
            ),

          // favorite indicator dot on image (top right) when favorite
          if (asset.isFavorite)
            Positioned(
              top: 12,
              right: 12,
              child: Container(
                width: 32,
                height: 32,
                decoration: BoxDecoration(
                  color: const Color(0xFFFF3B5C),
                  shape: BoxShape.circle,
                  boxShadow: [BoxShadow(color: Colors.black.withValues(alpha: 0.3), blurRadius: 8)],
                ),
                child: const Icon(Icons.favorite_rounded, size: 16, color: Colors.white),
              ),
            ),

          // bottom gradient + info row
          Positioned(
            bottom: 0,
            left: 0,
            right: 0,
            child: Container(
              decoration: BoxDecoration(
                gradient: LinearGradient(
                  begin: Alignment.topCenter,
                  end: Alignment.bottomCenter,
                  colors: [Colors.transparent, Colors.black.withValues(alpha: 0.72)],
                ),
              ),
              padding: const EdgeInsets.fromLTRB(14, 28, 14, 12),
              child: Row(children: [
                Expanded(
                  child: Column(crossAxisAlignment: CrossAxisAlignment.start, mainAxisSize: MainAxisSize.min, children: [
                    Text(asset.originalFileName,
                        style: const TextStyle(color: Colors.white, fontWeight: FontWeight.w700, fontSize: 13),
                        overflow: TextOverflow.ellipsis,
                        maxLines: 1),
                    const SizedBox(height: 2),
                    Row(children: [
                      const Icon(Icons.calendar_today_rounded, size: 11, color: Colors.white70),
                      const SizedBox(width: 4),
                      Text(dateLabel, style: const TextStyle(color: Colors.white70, fontSize: 11)),
                      if (asset.exifInfo?.make != null) ...[
                        const SizedBox(width: 8),
                        const Icon(Icons.camera_alt_rounded, size: 11, color: Colors.white70),
                        const SizedBox(width: 4),
                        Flexible(
                          child: Text(
                            [asset.exifInfo?.make, asset.exifInfo?.model].whereType<String>().join(' '),
                            style: const TextStyle(color: Colors.white70, fontSize: 11),
                            overflow: TextOverflow.ellipsis,
                          ),
                        ),
                      ],
                    ]),
                  ]),
                ),
                const SizedBox(width: 10),
                _circleBtn(
                  icon: asset.isFavorite ? Icons.favorite_rounded : Icons.favorite_border_rounded,
                  color: asset.isFavorite ? const Color(0xFFFF3B5C) : Colors.white,
                  bg: asset.isFavorite ? Colors.white : Colors.white.withValues(alpha: 0.18),
                  onTap: onFavoriteToggle,
                ),
                const SizedBox(width: 8),
                _circleBtn(
                  icon: Icons.info_outline_rounded,
                  color: Colors.white,
                  bg: Colors.white.withValues(alpha: 0.18),
                  onTap: onInfo,
                ),
              ]),
            ),
          ),
        ]),
      ),
    );
  }

  Widget _circleBtn({required IconData icon, required Color color, required Color bg, VoidCallback? onTap}) {
    return Material(
      color: bg,
      shape: const CircleBorder(),
      child: InkWell(
        customBorder: const CircleBorder(),
        onTap: onTap,
        child: Container(
          width: 40,
          height: 40,
          alignment: Alignment.center,
          child: Icon(icon, size: 18, color: color),
        ),
      ),
    );
  }

  String _formatDate(String raw) {
    if (raw.isEmpty) return '';
    try {
      final d = DateTime.parse(raw);
      const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
      return '${months[d.month - 1]} ${d.day}, ${d.year}';
    } catch (_) {
      return raw.length > 10 ? raw.substring(0, 10) : raw;
    }
  }
}
