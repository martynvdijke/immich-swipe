// ignore_for_file: deprecated_member_use
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import '../../../models/immich_album.dart';

/// Pure presentation chip bar for quick album assignment (0-9).
class AlbumChips extends StatefulWidget {
  const AlbumChips({super.key, required this.albums, required this.onSelect});
  final List<ImmichAlbum> albums;
  final void Function(ImmichAlbum) onSelect;

  @override
  State<AlbumChips> createState() => _AlbumChipsState();
}

class _AlbumChipsState extends State<AlbumChips> {
  String? _justAssignedId;

  void _handleSelect(ImmichAlbum album) {
    HapticFeedback.selectionClick();
    setState(() => _justAssignedId = album.id);
    widget.onSelect(album);
    Future.delayed(const Duration(milliseconds: 1400), () {
      if (mounted && _justAssignedId == album.id) {
        setState(() => _justAssignedId = null);
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    if (widget.albums.isEmpty) return const SizedBox.shrink();
    final shown = widget.albums.take(10).toList();
    final scheme = Theme.of(context).colorScheme;
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        Padding(
          padding: const EdgeInsets.only(left: 4, bottom: 6),
          child: Row(children: [
            Icon(Icons.collections_outlined, size: 14, color: scheme.onSurfaceVariant),
            const SizedBox(width: 6),
            Text(
              'Quick albums',
              style: Theme.of(context).textTheme.labelSmall?.copyWith(
                    color: scheme.onSurfaceVariant,
                    fontWeight: FontWeight.w600,
                    letterSpacing: 0.3,
                  ),
            ),
            const SizedBox(width: 6),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
              decoration: BoxDecoration(
                color: scheme.secondaryContainer,
                borderRadius: BorderRadius.circular(6),
              ),
              child: Text('0–9',
                  style: Theme.of(context).textTheme.labelSmall?.copyWith(
                        fontSize: 10,
                        fontWeight: FontWeight.w800,
                        color: scheme.onSecondaryContainer,
                      )),
            ),
          ]),
        ),
        SingleChildScrollView(
          scrollDirection: Axis.horizontal,
          padding: const EdgeInsets.symmetric(horizontal: 2, vertical: 2),
          child: Row(children: [
            for (int i = 0; i < shown.length; i++)
              Padding(
                padding: const EdgeInsets.only(right: 8),
                child: _AlbumChip(
                  index: i,
                  album: shown[i],
                  isAssigned: _justAssignedId == shown[i].id,
                  isDark: isDark,
                  scheme: scheme,
                  onTap: () => _handleSelect(shown[i]),
                ),
              ),
          ]),
        ),
      ],
    );
  }
}

class _AlbumChip extends StatelessWidget {
  const _AlbumChip({
    required this.index,
    required this.album,
    required this.isAssigned,
    required this.isDark,
    required this.scheme,
    required this.onTap,
  });
  final int index;
  final ImmichAlbum album;
  final bool isAssigned;
  final bool isDark;
  final ColorScheme scheme;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    // Keep ActionChip type for test compatibility; style it heavily.
    return ActionChip(
      label: Row(mainAxisSize: MainAxisSize.min, children: [
        Container(
          width: 22,
          height: 22,
          alignment: Alignment.center,
          decoration: BoxDecoration(
            color: isAssigned ? scheme.primary : (isDark ? Colors.white.withOpacity(0.12) : scheme.primaryContainer),
            borderRadius: BorderRadius.circular(7),
          ),
          child: Text(
            '$index',
            style: TextStyle(
              fontSize: 11,
              fontWeight: FontWeight.w800,
              color: isAssigned ? scheme.onPrimary : (isDark ? Colors.white : scheme.onPrimaryContainer),
            ),
          ),
        ),
        const SizedBox(width: 8),
        Flexible(
          child: Text(
            album.albumName,
            overflow: TextOverflow.ellipsis,
            style: TextStyle(
              fontWeight: FontWeight.w600,
              fontSize: 13,
              color: isAssigned ? scheme.primary : scheme.onSurface,
            ),
          ),
        ),
        if (isAssigned) ...[
          const SizedBox(width: 6),
          Icon(Icons.check_circle, size: 14, color: scheme.primary),
        ],
      ]),
      onPressed: onTap,
      backgroundColor: isAssigned
          ? scheme.primaryContainer.withOpacity(isDark ? 0.35 : 1)
          : (isDark ? Colors.white.withOpacity(0.07) : Colors.white),
      side: BorderSide(
        color: isAssigned ? scheme.primary.withOpacity(0.6) : scheme.outlineVariant.withOpacity(isDark ? 0.2 : 0.6),
      ),
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 6),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(20)),
    );
  }
}
