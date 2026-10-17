// ignore_for_file: deprecated_member_use
import 'package:flutter/material.dart';
import '../../../core/format.dart';

class StatsBar extends StatelessWidget {
  const StatsBar({super.key, required this.kept, required this.trashed, required this.freedBytes});
  final int kept;
  final int trashed;
  final int freedBytes;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      decoration: BoxDecoration(
        color: isDark ? Colors.white.withOpacity(0.06) : Colors.white,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: scheme.outlineVariant.withOpacity(isDark ? 0.15 : 0.5)),
      ),
      child: Row(children: [
        _pill(
          context,
          icon: Icons.check_rounded,
          label: 'Kept: $kept',
          color: const Color(0xFF10B981),
          scheme: scheme,
          isDark: isDark,
        ),
        const SizedBox(width: 8),
        _pill(
          context,
          icon: Icons.delete_outline_rounded,
          label: 'Trashed: $trashed',
          color: const Color(0xFFEF4444),
          scheme: scheme,
          isDark: isDark,
        ),
        const Spacer(),
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
          decoration: BoxDecoration(
            color: scheme.secondaryContainer.withOpacity(isDark ? 0.5 : 1),
            borderRadius: BorderRadius.circular(12),
          ),
          child: Row(mainAxisSize: MainAxisSize.min, children: [
            Icon(Icons.sd_storage_rounded, size: 14, color: scheme.onSecondaryContainer),
            const SizedBox(width: 6),
            Text(
              'Freed: ${formatBytes(freedBytes)}',
              style: Theme.of(context).textTheme.labelMedium?.copyWith(
                    fontWeight: FontWeight.w700,
                    color: scheme.onSecondaryContainer,
                    fontSize: 12,
                  ),
            ),
          ]),
        ),
      ]),
    );
  }

  Widget _pill(BuildContext context,
      {required IconData icon, required String label, required Color color, required ColorScheme scheme, required bool isDark}) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
      decoration: BoxDecoration(
        color: color.withOpacity(isDark ? 0.18 : 0.12),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: color.withOpacity(isDark ? 0.3 : 0.25)),
      ),
      child: Row(mainAxisSize: MainAxisSize.min, children: [
        Icon(icon, size: 14, color: color),
        const SizedBox(width: 6),
        Text(
          label,
          style: Theme.of(context).textTheme.labelMedium?.copyWith(
                fontWeight: FontWeight.w700,
                color: isDark ? Colors.white : const Color(0xFF1F2937),
                fontSize: 12,
              ),
        ),
      ]),
    );
  }
}
