String formatBytes(int bytes) {
  if (bytes < 1024) return '$bytes B';
  if (bytes < 1024 * 1024) return '${(bytes / 1024).toStringAsFixed(1)} KB';
  if (bytes < 1024 * 1024 * 1024) {
    return '${(bytes / (1024 * 1024)).toStringAsFixed(1)} MB';
  }
  return '${(bytes / (1024 * 1024 * 1024)).toStringAsFixed(2)} GB';
}

String formatCount(int count) => count.toString();

/// Immich reports video length as `H:MM:SS.ffffff` (e.g. `0:00:12.000000`).
/// Render it the way a player would: `12:05`, `0:42`.
String formatDuration(String? raw) {
  if (raw == null || raw.trim().isEmpty) return 'Video';
  final parts = raw.trim().split(':');
  if (parts.length < 2) return raw;
  final secs = double.tryParse(parts.last)?.floor() ?? 0;
  final mins = int.tryParse(parts[parts.length - 2]) ?? 0;
  final hours = parts.length > 2 ? (int.tryParse(parts[parts.length - 3]) ?? 0) : 0;
  final total = hours * 3600 + mins * 60 + secs;
  if (total <= 0) return 'Video';
  return '${total ~/ 60}:${(total % 60).toString().padLeft(2, '0')}';
}
