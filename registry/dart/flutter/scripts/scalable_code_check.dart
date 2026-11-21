import 'dart:convert';
import 'dart:io';

Future<void> main(List<String> args) async {
  final options = _parseArgs(args);
  final configFile = File('.scalable-code/config.json');

  if (!configFile.existsSync()) {
    stderr.writeln(
      'Error: .scalable-code/config.json not found. Run /scalable-code-setup first.',
    );
    exit(2);
  }

  final config = jsonDecode(configFile.readAsStringSync()) as Map<String, dynamic>;
  final targetDir = (config['targetDir'] as String?) ?? 'lib';
  final scopedFiles = _resolveScope(options, targetDir);

  if (!options.all && scopedFiles.isEmpty) {
    final emptyReport = _buildReport(
      mode: 'changed',
      scopedFiles: const [],
      findings: const [],
      lakosMetrics: const {},
      toolStatuses: {'scope': 'No changed Dart files detected under $targetDir.'},
    );
    _writeReports(emptyReport);
    stdout.writeln('No changed Dart files under $targetDir. Skipping checks (0 findings).');
    exit(0);
  }

  final findings = <_Finding>[];
  final toolStatuses = <String, String>{};

  await _runDartAnalyze(scopedFiles, options.all, findings, toolStatuses);
  await _runCustomLint(scopedFiles, options.all, findings, toolStatuses);
  await _runDartCodeLinter(targetDir, scopedFiles, options.all, findings, toolStatuses);
  final lakosMetrics = await _runLakos(
    targetDir,
    config,
    scopedFiles,
    options.all,
    findings,
    toolStatuses,
  );
  await _runMutationTest(
    targetDir,
    config,
    scopedFiles,
    options.all,
    options.skipMutation,
    findings,
    toolStatuses,
  );

  final report = _buildReport(
    mode: options.all ? 'all' : 'changed',
    scopedFiles: scopedFiles.toList()..sort(),
    findings: findings,
    lakosMetrics: lakosMetrics,
    toolStatuses: toolStatuses,
  );
  _writeReports(report);

  final blockingCount = findings.where((f) => f.severity == 'error' || f.severity == 'warning').length;
  stdout.writeln(
    'scalable_code_check (${options.all ? 'all' : 'changed'}): '
    '${findings.length} finding(s) ($blockingCount blocking). '
    'Report saved to .scalable-code/report.md',
  );

  exit(blockingCount > 0 ? 1 : 0);
}

class _CliOptions {
  _CliOptions({
    required this.all,
    required this.explicitFiles,
    required this.skipMutation,
  });
  final bool all;
  final List<String> explicitFiles;
  final bool skipMutation;
}

class _Finding {
  _Finding({
    required this.tool,
    required this.severity,
    required this.rule,
    required this.message,
    this.file,
    this.line,
  });

  final String tool;
  final String severity;
  final String rule;
  final String message;
  final String? file;
  final int? line;

  Map<String, dynamic> toJson() => {
        'tool': tool,
        'severity': severity,
        'rule': rule,
        'message': message,
        if (file != null) 'file': file,
        if (line != null) 'line': line,
      };
}

_CliOptions _parseArgs(List<String> args) {
  var all = false;
  var skipMutation = false;
  final explicitFiles = <String>[];
  for (var i = 0; i < args.length; i++) {
    final arg = args[i];
    if (arg == '--all') {
      all = true;
    } else if (arg == '--changed') {
      all = false;
    } else if (arg == '--no-mutation' || arg == '--skip-mutation') {
      skipMutation = true;
    } else if (arg == '--files' && i + 1 < args.length) {
      explicitFiles.addAll(
        args[++i].split(',').map((s) => _normalizePath(s.trim())).where((s) => s.isNotEmpty),
      );
    } else if (arg.startsWith('--files=')) {
      explicitFiles.addAll(
        arg.substring('--files='.length).split(',').map((s) => _normalizePath(s.trim())).where((s) => s.isNotEmpty),
      );
    }
  }
  return _CliOptions(all: all, explicitFiles: explicitFiles, skipMutation: skipMutation);
}

Set<String> _resolveScope(_CliOptions options, String targetDir) {
  if (options.explicitFiles.isNotEmpty) {
    return options.explicitFiles.where((f) => f.endsWith('.dart')).toSet();
  }
  if (options.all) {
    return _listDartFiles(targetDir);
  }
  return _gitChangedDartFiles(targetDir);
}

Set<String> _listDartFiles(String targetDir) {
  final dir = Directory(targetDir);
  if (!dir.existsSync()) return {};
  final files = <String>{};
  for (final entity in dir.listSync(recursive: true)) {
    if (entity is File && entity.path.endsWith('.dart')) {
      files.add(_normalizePath(entity.path));
    }
  }
  return files;
}

Set<String> _gitChangedDartFiles(String targetDir) {
  final statusRes = Process.runSync('git', ['status', '--porcelain', '-uall']);
  if (statusRes.exitCode != 0) {
    // Fallback to full targetDir when directory is not a git repository.
    return _listDartFiles(targetDir);
  }

  final normalizedPrefix = '${_normalizePath(targetDir).replaceAll(RegExp(r'/+$'), '')}/';
  final changed = <String>{};

  for (final rawLine in (statusRes.stdout as String).split('\n')) {
    if (rawLine.trim().isEmpty || rawLine.length < 4) continue;
    final statusCode = rawLine.substring(0, 2);
    if (statusCode.contains('D')) continue;

    var pathPart = rawLine.substring(3).trim();
    if (pathPart.contains(' -> ')) {
      pathPart = pathPart.split(' -> ').last.trim();
    }
    pathPart = pathPart.replaceAll('"', '');
    final normalized = _normalizePath(pathPart);
    if (normalized.endsWith('.dart') && normalized.startsWith(normalizedPrefix)) {
      changed.add(normalized);
    }
  }
  return changed;
}

Future<void> _runDartAnalyze(
  Set<String> scopedFiles,
  bool all,
  List<_Finding> findings,
  Map<String, String> statuses,
) async {
  final args = ['analyze', '--format=json'];
  if (!all && scopedFiles.isNotEmpty) {
    args.addAll(scopedFiles);
  }
  final result = await Process.run('dart', args);
  final stdoutStr = (result.stdout as String).trim();
  final stderrStr = (result.stderr as String).trim();

  final parsed = _tryDecodeJson(stdoutStr);
  if (parsed is Map<String, dynamic> && parsed['diagnostics'] is List) {
    final diags = parsed['diagnostics'] as List;
    for (final item in diags) {
      if (item is! Map<String, dynamic>) continue;
      final loc = item['location'] as Map<String, dynamic>?;
      final file = loc != null ? _normalizePath((loc['file'] as String?) ?? '') : null;
      if (!all && file != null && !_matchesScope(file, scopedFiles)) continue;

      final severityRaw = ((item['severity'] as String?) ?? 'warning').toLowerCase();
      final code = (item['code'] as String?) ?? 'analyze';
      final severity = (severityRaw == 'info' || code == 'analysis_options_deprecated_plugins')
          ? 'info'
          : (severityRaw == 'error' ? 'error' : 'warning');
      final range = loc?['range'] as Map<String, dynamic>?;
      final start = range?['start'] as Map<String, dynamic>?;
      findings.add(
        _Finding(
          tool: 'dart_analyze',
          severity: severity,
          rule: code,
          message: (item['problemMessage'] as String?) ?? 'Analyzer diagnostic',
          file: file,
          line: start?['line'] as int?,
        ),
      );
    }
    statuses['dart_analyze'] = 'ok (${diags.length} diagnostic(s) inspected)';
    return;
  }

  if (result.exitCode != 0) {
    findings.add(
      _Finding(
        tool: 'dart_analyze',
        severity: 'error',
        rule: 'dart-analyze',
        message: stdoutStr.isNotEmpty ? stdoutStr : stderrStr,
      ),
    );
    statuses['dart_analyze'] = 'failed (exit ${result.exitCode})';
  } else {
    statuses['dart_analyze'] = 'ok (0 issues)';
  }
}

Future<void> _runCustomLint(
  Set<String> scopedFiles,
  bool all,
  List<_Finding> findings,
  Map<String, String> statuses,
) async {
  if (!_isPackageDeclared('custom_lint')) {
    statuses['custom_lint'] = 'skipped (not in pubspec.yaml; native plugins run via dart analyze)';
    return;
  }

  final result = await Process.run('dart', ['run', 'custom_lint']);
  final output = '${result.stdout}\n${result.stderr}'.trim();

  if (result.exitCode == 0) {
    statuses['custom_lint'] = 'ok (0 issues)';
    return;
  }

  var parsedCount = 0;
  final lineRegex = RegExp(r'^\s*([^\s:]+):(\d+):(\d+)\s+•\s+(.+?)\s+•\s+([^\s]+)\s+•\s+(\w+)\s*$');
  for (final line in output.split('\n')) {
    final match = lineRegex.firstMatch(line.trimRight());
    if (match == null) continue;
    final file = _normalizePath(match.group(1)!);
    if (!all && !_matchesScope(file, scopedFiles)) continue;

    parsedCount++;
    findings.add(
      _Finding(
        tool: 'custom_lint',
        severity: match.group(6)!.toLowerCase() == 'error' ? 'error' : 'warning',
        rule: match.group(5)!,
        message: match.group(4)!,
        file: file,
        line: int.tryParse(match.group(2)!),
      ),
    );
  }

  if (parsedCount == 0 && output.isNotEmpty) {
    findings.add(
      _Finding(
        tool: 'custom_lint',
        severity: 'warning',
        rule: 'custom_lint',
        message: output,
      ),
    );
  }
  statuses['custom_lint'] = 'completed ($parsedCount issue(s) in scope)';
}

Future<void> _runDartCodeLinter(
  String targetDir,
  Set<String> scopedFiles,
  bool all,
  List<_Finding> findings,
  Map<String, String> statuses,
) async {
  final result = await Process.run(
    'dart',
    ['run', 'dart_code_linter:metrics', 'analyze', targetDir, '--reporter=json'],
  );
  final stdoutStr = (result.stdout as String).trim();
  final stderrStr = (result.stderr as String).trim();

  final decoded = _tryDecodeJson(stdoutStr);
  if (decoded is Map<String, dynamic> && decoded['records'] is List) {
    final records = decoded['records'] as List;
    var matchedCount = 0;

    for (final rec in records) {
      if (rec is! Map<String, dynamic>) continue;
      final filePath = _normalizePath((rec['path'] as String?) ?? '');
      if (filePath.isEmpty) continue;
      if (!all && !_matchesScope(filePath, scopedFiles)) continue;

      matchedCount += _collectDclEntries(rec['issues'], filePath, 'rule', findings);
      matchedCount += _collectDclEntries(rec['antiPatternCases'], filePath, 'anti-pattern', findings);
      matchedCount += _collectDclMetricList(rec['fileMetrics'], filePath, null, findings);
      matchedCount += _collectDclEntityMetrics(rec['classes'], filePath, findings);
      matchedCount += _collectDclEntityMetrics(rec['functions'], filePath, findings);
      matchedCount += _collectDclReportsLegacy(rec['reports'], filePath, findings);
    }

    statuses['dart_code_linter'] = 'ok ($matchedCount finding(s) in scope)';
    return;
  }

  if (result.exitCode != 0 && stdoutStr.isEmpty) {
    findings.add(
      _Finding(
        tool: 'dart_code_linter',
        severity: 'error',
        rule: 'dcl-execution',
        message: stderrStr.isNotEmpty ? stderrStr : 'dart_code_linter failed with exit ${result.exitCode}',
      ),
    );
    statuses['dart_code_linter'] = 'failed (exit ${result.exitCode})';
  } else {
    statuses['dart_code_linter'] = 'ok (no structured records)';
  }
}

int _collectDclEntries(
  dynamic entries,
  String filePath,
  String defaultKind,
  List<_Finding> findings,
) {
  if (entries is! List) return 0;
  var count = 0;
  for (final item in entries) {
    if (item is! Map<String, dynamic>) continue;
    final span = (item['codeSpan'] as Map<String, dynamic>?) ?? (item['location'] as Map<String, dynamic>?);
    final start = span?['start'] as Map<String, dynamic>?;
    final rawSeverity = ((item['severity'] as String?) ?? 'warning').toLowerCase();
    final severity = rawSeverity == 'none'
        ? 'info'
        : (rawSeverity == 'style' ? 'warning' : rawSeverity);
    findings.add(
      _Finding(
        tool: 'dart_code_linter',
        severity: severity,
        rule: (item['ruleId'] as String?) ?? defaultKind,
        message: (item['message'] as String?) ?? 'DCL violation',
        file: filePath,
        line: start?['line'] as int?,
      ),
    );
    count++;
  }
  return count;
}

int _collectDclEntityMetrics(
  dynamic entityMap,
  String filePath,
  List<_Finding> findings,
) {
  if (entityMap is! Map<String, dynamic>) return 0;
  var count = 0;
  entityMap.forEach((_, rawEntity) {
    if (rawEntity is! Map<String, dynamic>) return;
    final span = rawEntity['codeSpan'] as Map<String, dynamic>?;
    final start = span?['start'] as Map<String, dynamic>?;
    count += _collectDclMetricList(
      rawEntity['metrics'],
      filePath,
      start?['line'] as int?,
      findings,
    );
  });
  return count;
}

int _collectDclReportsLegacy(
  dynamic reports,
  String filePath,
  List<_Finding> findings,
) {
  if (reports is! List) return 0;
  var count = 0;
  for (final rep in reports) {
    if (rep is! Map<String, dynamic>) continue;
    final loc = rep['location'] as Map<String, dynamic>?;
    final start = loc?['start'] as Map<String, dynamic>?;
    count += _collectDclMetricList(rep['metrics'], filePath, start?['line'] as int?, findings);
  }
  return count;
}

int _collectDclMetricList(
  dynamic metrics,
  String filePath,
  int? line,
  List<_Finding> findings,
) {
  if (metrics is! List) return 0;
  var count = 0;
  for (final m in metrics) {
    if (m is! Map<String, dynamic>) continue;
    final level = ((m['level'] as String?) ?? 'none').toLowerCase();
    if (level == 'none' || level == 'noted') continue;
    final metricId = (m['metricsId'] as String?) ?? 'metric';
    final value = m['value'];
    final comment = (m['comment'] as String?) ?? '$metricId exceeded threshold ($value)';
    findings.add(
      _Finding(
        tool: 'dart_code_linter',
        severity: level == 'alarm' || level == 'error' ? 'error' : 'warning',
        rule: metricId,
        message: comment,
        file: filePath,
        line: line,
      ),
    );
    count++;
  }
  return count;
}

Future<Map<String, dynamic>> _runLakos(
  String targetDir,
  Map<String, dynamic> config,
  Set<String> scopedFiles,
  bool all,
  List<_Finding> findings,
  Map<String, String> statuses,
) async {
  final args = ['run', 'lakos', '-f', 'json', '-m', '--node-metrics'];
  final excludeGlobs = (config['excludeGlobs'] as List?)?.cast<String>() ?? const <String>[];
  if (excludeGlobs.isNotEmpty) {
    args.add('-i={${excludeGlobs.join(',')}}');
  }
  args.add(targetDir);

  final result = await Process.run('dart', args);
  final stdoutStr = (result.stdout as String).trim();
  final decoded = _tryDecodeJson(stdoutStr);

  if (decoded is! Map<String, dynamic>) {
    findings.add(
      _Finding(
        tool: 'lakos',
        severity: 'error',
        rule: 'lakos-execution',
        message: (result.stderr as String).trim().isNotEmpty
            ? (result.stderr as String).trim()
            : 'Failed to parse lakos JSON output.',
      ),
    );
    statuses['lakos'] = 'failed';
    return const {};
  }

  final lakosConfig = (config['lakos'] as Map<String, dynamic>?) ?? const {};
  final allowCycles = (lakosConfig['allowCycles'] as bool?) ?? false;
  final maxOutDegree = lakosConfig['maxOutDegree'] as int?;
  final maxAcd = (lakosConfig['maxAcd'] as num?)?.toDouble();

  final metrics = (decoded['metrics'] as Map<String, dynamic>?) ?? const {};
  final isAcyclic = (metrics['isAcyclic'] as bool?) ?? true;
  final acd = (metrics['acd'] as num?)?.toDouble();

  if (!allowCycles && !isAcyclic) {
    findings.add(
      _Finding(
        tool: 'lakos',
        severity: 'error',
        rule: 'no-dependency-cycles',
        message: 'Circular dependencies detected in $targetDir (metrics.isAcyclic == false).',
      ),
    );
  }

  if (maxAcd != null && acd != null && acd > maxAcd) {
    findings.add(
      _Finding(
        tool: 'lakos',
        severity: 'warning',
        rule: 'max-acd',
        message: 'Global Average Component Dependency (ACD=${acd.toStringAsFixed(2)}) exceeds threshold $maxAcd.',
      ),
    );
  }

  final nodes = (decoded['nodes'] as Map<String, dynamic>?) ?? const {};
  if (maxOutDegree != null) {
    nodes.forEach((nodeId, rawNode) {
      if (rawNode is! Map<String, dynamic>) return;
      final filePath = _lakosNodeToPath(targetDir, nodeId);
      if (!all && !_matchesScope(filePath, scopedFiles)) return;

      final outDegree = rawNode['outDegree'] as int?;
      if (outDegree != null && outDegree > maxOutDegree) {
        findings.add(
          _Finding(
            tool: 'lakos',
            severity: 'warning',
            rule: 'max-out-degree',
            message: 'File imports $outDegree internal modules (maxOutDegree=$maxOutDegree); consider splitting responsibilities.',
            file: filePath,
          ),
        );
      }
    });
  }

  _checkArchitectureBoundaries(
    targetDir: targetDir,
    edges: (decoded['edges'] as List?) ?? const [],
    config: config,
    scopedFiles: scopedFiles,
    all: all,
    findings: findings,
  );

  statuses['lakos'] = 'ok (isAcyclic=$isAcyclic, ccd=${metrics['ccd']}, acd=${metrics['acd']})';
  return metrics;
}

Future<void> _runMutationTest(
  String targetDir,
  Map<String, dynamic> config,
  Set<String> scopedFiles,
  bool all,
  bool skipMutation,
  List<_Finding> findings,
  Map<String, String> statuses,
) async {
  if (skipMutation) {
    statuses['mutation_test'] = 'skipped (--no-mutation)';
    return;
  }
  if (!_isPackageDeclared('mutation_test')) {
    statuses['mutation_test'] = 'skipped (not in pubspec.yaml)';
    return;
  }

  final mutationConfig = (config['mutationTest'] as Map<String, dynamic>?) ?? const {};
  final enabled = (mutationConfig['enabled'] as bool?) ?? true;
  if (!enabled) {
    statuses['mutation_test'] = 'skipped (disabled in .scalable-code/config.json)';
    return;
  }

  if (!_hasTestFiles()) {
    statuses['mutation_test'] = 'skipped (no test files found under test/)';
    return;
  }

  final excludeGlobs = (config['excludeGlobs'] as List?)?.cast<String>() ?? const <String>[];
  final candidateFiles = all ? _listDartFiles(targetDir) : scopedFiles;
  final targetFiles = candidateFiles.where((f) => !_isGeneratedOrExcluded(f, excludeGlobs)).toList()..sort();

  if (targetFiles.isEmpty) {
    statuses['mutation_test'] = 'skipped (no non-generated files in scope)';
    return;
  }

  final threshold = (mutationConfig['threshold'] as num?)?.toInt() ?? 80;
  final testCommand = (mutationConfig['testCommand'] as String?) ?? 'flutter test';
  final timeoutSeconds = (mutationConfig['timeoutSeconds'] as num?)?.toInt();

  final outDir = Directory('.scalable-code');
  if (!outDir.existsSync()) {
    outDir.createSync(recursive: true);
  }

  final xmlBuffer = StringBuffer()
    ..writeln('<mutations version="1.1">')
    ..writeln('  <files>');
  for (final file in targetFiles) {
    xmlBuffer.writeln('    <file>${_escapeXml(file)}</file>');
  }
  xmlBuffer
    ..writeln('  </files>')
    ..writeln('  <commands>');
  final timeoutAttr = timeoutSeconds != null ? ' timeout="$timeoutSeconds"' : '';
  xmlBuffer
    ..writeln('    <command group="test" expected-return="0"$timeoutAttr>${_escapeXml(testCommand)}</command>')
    ..writeln('  </commands>')
    ..writeln('  <threshold failure="$threshold">')
    ..writeln('    <rating over="$threshold" name="pass" />')
    ..writeln('    <rating over="0" name="fail" />')
    ..writeln('  </threshold>')
    ..writeln('</mutations>');

  const configPath = '.scalable-code/mutation-config.xml';
  const reportDir = '.scalable-code/mutation-report';
  File(configPath).writeAsStringSync(xmlBuffer.toString());

  final result = await Process.run(
    'dart',
    ['run', 'mutation_test', configPath, '-o', reportDir],
  );
  final stdoutStr = (result.stdout as String).trim();
  final stderrStr = (result.stderr as String).trim();
  final combinedOutput = '$stdoutStr\n$stderrStr'.trim();

  final scoreMatch = RegExp(r'(\d+(?:\.\d+)?)\s*%').allMatches(combinedOutput).lastOrNull;
  final scoreLabel = scoreMatch != null ? '${scoreMatch.group(1)}%' : 'unknown';

  if (result.exitCode == 0) {
    statuses['mutation_test'] = 'ok (score=$scoreLabel, threshold=$threshold%, files=${targetFiles.length})';
    return;
  }

  findings.add(
    _Finding(
      tool: 'mutation_test',
      severity: 'warning',
      rule: 'mutation-score-threshold',
      message: 'Mutation testing failed (score=$scoreLabel, threshold=$threshold%). '
          'Review surviving mutants in $reportDir/index.html.',
    ),
  );
  statuses['mutation_test'] = 'failed (exit ${result.exitCode}, score=$scoreLabel, threshold=$threshold%)';
}

bool _hasTestFiles() {
  final testDir = Directory('test');
  if (!testDir.existsSync()) return false;
  for (final entity in testDir.listSync(recursive: true)) {
    if (entity is File && entity.path.endsWith('_test.dart')) {
      return true;
    }
  }
  return false;
}

bool _isGeneratedOrExcluded(String filePath, List<String> excludeGlobs) {
  final norm = _normalizePath(filePath);
  if (norm.endsWith('.g.dart') || norm.endsWith('.freezed.dart') || norm.endsWith('.gr.dart')) {
    return true;
  }
  for (final glob in excludeGlobs) {
    final clean = glob.replaceAll('**/', '').replaceAll('*', '');
    if (clean.isNotEmpty && norm.endsWith(clean)) {
      return true;
    }
  }
  return false;
}

String _escapeXml(String value) {
  return value
      .replaceAll('&', '&amp;')
      .replaceAll('<', '&lt;')
      .replaceAll('>', '&gt;')
      .replaceAll('"', '&quot;')
      .replaceAll("'", '&apos;');
}

void _checkArchitectureBoundaries({
  required String targetDir,
  required List<dynamic> edges,
  required Map<String, dynamic> config,
  required Set<String> scopedFiles,
  required bool all,
  required List<_Finding> findings,
}) {
  final boundaries = (config['boundaries'] as Map<String, dynamic>?) ?? const {};
  final forbidden = (boundaries['forbiddenDependencies'] as List?) ?? const [];
  final featureIso = (boundaries['featureIsolation'] as Map<String, dynamic>?) ?? const {};
  final featureIsoEnabled = (featureIso['enabled'] as bool?) ?? false;
  final featuresPath = ((featureIso['featuresPath'] as String?) ?? 'features').replaceAll('/', '');

  for (final rawEdge in edges) {
    if (rawEdge is! Map<String, dynamic>) continue;
    final fromId = rawEdge['from'] as String?;
    final toId = rawEdge['to'] as String?;
    if (fromId == null || toId == null) continue;

    final fromPath = _lakosNodeToPath(targetDir, fromId);
    final toPath = _lakosNodeToPath(targetDir, toId);

    if (!all && !_matchesScope(fromPath, scopedFiles) && !_matchesScope(toPath, scopedFiles)) {
      continue;
    }

    for (final rule in forbidden) {
      if (rule is! Map<String, dynamic>) continue;
      final fromLayer = (rule['from'] as String?) ?? '';
      final toLayer = (rule['to'] as String?) ?? '';
      if (fromLayer.isEmpty || toLayer.isEmpty) continue;

      if (_pathHasSegment(fromPath, fromLayer) && _pathHasSegment(toPath, toLayer)) {
        final reason = (rule['reason'] as String?) ?? '$fromLayer cannot depend on $toLayer.';
        findings.add(
          _Finding(
            tool: 'architecture',
            severity: 'error',
            rule: 'forbidden-layer-dependency ($fromLayer -> $toLayer)',
            message: '$reason ($fromPath -> $toPath)',
            file: fromPath,
          ),
        );
      }
    }

    if (featureIsoEnabled) {
      final fromFeature = _extractFeatureName(fromPath, featuresPath);
      final toFeature = _extractFeatureName(toPath, featuresPath);
      if (fromFeature != null && toFeature != null && fromFeature != toFeature) {
        findings.add(
          _Finding(
            tool: 'architecture',
            severity: 'error',
            rule: 'feature-isolation',
            message: 'Feature "$fromFeature" directly imports feature "$toFeature" ($fromPath -> $toPath).',
            file: fromPath,
          ),
        );
      }
    }
  }
}

String _lakosNodeToPath(String targetDir, String nodeId) {
  final cleanTarget = _normalizePath(targetDir).replaceAll(RegExp(r'/+$'), '');
  final cleanNode = _normalizePath(nodeId).replaceFirst(RegExp(r'^/+'), '');
  if (cleanNode.startsWith('$cleanTarget/')) return cleanNode;
  return '$cleanTarget/$cleanNode';
}

bool _pathHasSegment(String filePath, String segment) {
  final parts = _normalizePath(filePath).split('/');
  final target = segment.replaceAll('/', '');
  return parts.contains(target);
}

String? _extractFeatureName(String filePath, String featuresSegment) {
  final parts = _normalizePath(filePath).split('/');
  final idx = parts.indexOf(featuresSegment);
  if (idx == -1 || idx + 1 >= parts.length) return null;
  return parts[idx + 1];
}

bool _matchesScope(String filePath, Set<String> scopedFiles) {
  final norm = _normalizePath(filePath);
  for (final s in scopedFiles) {
    if (norm == s || norm.endsWith('/$s') || s.endsWith('/$norm')) {
      return true;
    }
  }
  return false;
}

String _normalizePath(String path) => path.replaceAll('\\', '/');

bool _isPackageDeclared(String packageName) {
  final pubspec = File('pubspec.yaml');
  if (!pubspec.existsSync()) return false;
  final pattern = RegExp('^\\s*${RegExp.escape(packageName)}\\s*:', multiLine: true);
  return pattern.hasMatch(pubspec.readAsStringSync());
}

dynamic _tryDecodeJson(String raw) {
  if (raw.isEmpty) return null;
  final startBrace = raw.indexOf('{');
  if (startBrace == -1) return null;
  try {
    return jsonDecode(raw.substring(startBrace));
  } catch (_) {
    return null;
  }
}

Map<String, dynamic> _buildReport({
  required String mode,
  required List<String> scopedFiles,
  required List<_Finding> findings,
  required Map<String, dynamic> lakosMetrics,
  required Map<String, String> toolStatuses,
}) {
  return {
    'generatedAt': DateTime.now().toUtc().toIso8601String(),
    'mode': mode,
    'scopedFiles': scopedFiles,
    'summary': {
      'totalFindings': findings.length,
      'errors': findings.where((f) => f.severity == 'error').length,
      'warnings': findings.where((f) => f.severity == 'warning').length,
      'info': findings.where((f) => f.severity == 'info').length,
    },
    'lakosMetrics': lakosMetrics,
    'toolStatuses': toolStatuses,
    'findings': findings.map((f) => f.toJson()).toList(),
  };
}

void _writeReports(Map<String, dynamic> report) {
  final outDir = Directory('.scalable-code');
  if (!outDir.existsSync()) {
    outDir.createSync(recursive: true);
  }

  const encoder = JsonEncoder.withIndent('  ');
  File('.scalable-code/report.json').writeAsStringSync('${encoder.convert(report)}\n');

  final md = StringBuffer()
    ..writeln('# Scalable Code Empirical Report')
    ..writeln()
    ..writeln('- **Generated at:** ${report['generatedAt']}')
    ..writeln('- **Mode:** ${report['mode']}')
    ..writeln('- **Scoped files:** ${(report['scopedFiles'] as List).length}');

  final summary = report['summary'] as Map<String, dynamic>;
  md
    ..writeln(
      '- **Summary:** ${summary['totalFindings']} total '
      '(${summary['errors']} errors, ${summary['warnings']} warnings, ${summary['info']} info)',
    )
    ..writeln();

  final lakosMetrics = report['lakosMetrics'] as Map<String, dynamic>;
  if (lakosMetrics.isNotEmpty) {
    md
      ..writeln('## Dependency Graph Metrics (lakos)')
      ..writeln('- **isAcyclic:** ${lakosMetrics['isAcyclic']}')
      ..writeln('- **CCD:** ${lakosMetrics['ccd']}')
      ..writeln('- **ACD:** ${lakosMetrics['acd']}')
      ..writeln();
  }

  final statuses = report['toolStatuses'] as Map<String, String>;
  if (statuses.isNotEmpty) {
    md.writeln('## Tool Statuses');
    statuses.forEach((k, v) => md.writeln('- **$k:** $v'));
    md.writeln();
  }

  final findings = report['findings'] as List<dynamic>;
  md.writeln('## Findings');
  if (findings.isEmpty) {
    md.writeln('No findings reported.');
  } else {
    for (final raw in findings) {
      final f = raw as Map<String, dynamic>;
      final location = f['file'] != null
          ? (f['line'] != null ? '${f['file']}:${f['line']}' : '${f['file']}')
          : 'project';
      md.writeln(
        '- **[${(f['severity'] as String).toUpperCase()}]** `${f['tool']}` '
        '(`${f['rule']}`) at `$location`: ${f['message']}',
      );
    }
  }

  File('.scalable-code/report.md').writeAsStringSync(md.toString());
}
