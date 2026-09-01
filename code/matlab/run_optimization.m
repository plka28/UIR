function result = run_optimization()
%RUN_OPTIMIZATION Load data, build the problem and run intlinprog.

% Load and validate all prepared CSV inputs before building solver arrays.
fprintf('Loading prepared input data...\n');
data = load_input_data();

fprintf('Filtering input data for route feasibility...\n');
[data, filterReport] = filter_optimization_input(data);
print_filter_report(filterReport);

fprintf('Validating input data...\n');
validation = validate_input_data(data);
data.validation = validation;

fprintf('Input data loaded and validated.\n');
fprintf('Waste sources: %d\n', height(data.wasteSources));
fprintf('Transfer stations: %d\n', height(data.transferStations));
fprintf('Plants: %d\n', height(data.plants));

if ~isempty(validation.warnings)
    fprintf('Validation warnings:\n');
    for i = 1:numel(validation.warnings)
        fprintf('- %s\n', validation.warnings(i));
    end
end

% Build the MILP arrays and keep maps/problemInfo for decoding.
fprintf('Building optimization problem...\n');
[f, intcon, A, b, Aeq, beq, lb, ub, maps, problemInfo] = build_problem(data);

% Run intlinprog through the dedicated solver wrapper.
fprintf('Solving optimization problem...\n');
[x, fval, exitflag, output] = solve_problem(f, intcon, A, b, Aeq, beq, lb, ub);

fprintf('Optimization summary:\n');
fprintf('  exitflag: %d\n', exitflag);
fprintf('  objective value: %.12g\n', fval);
fprintf('  solve time, seconds: %.3f\n', output.solveTimeSeconds);
fprintf('  variables: %d\n', problemInfo.nVariables);
if isfield(output, 'solverAlgorithm')
    fprintf('  solver algorithm: %s\n', char(output.solverAlgorithm));
end

% Convert the flat solution vector to output tables and save CSV results.
fprintf('Decoding solution...\n');
decoded = decode_solution(x, data, maps, problemInfo, fval);

fprintf('Saving optimization results...\n');
outputPaths = save_results(decoded);

fprintf('Saved results:\n');
fprintf('  %s\n', outputPaths.sourceAssignments);
fprintf('  %s\n', outputPaths.transferFlows);
fprintf('  %s\n', outputPaths.summary);

result = struct();
result.data = data;
result.problem = struct( ...
    'f', f, ...
    'intcon', intcon, ...
    'A', A, ...
    'b', b, ...
    'Aeq', Aeq, ...
    'beq', beq, ...
    'lb', lb, ...
    'ub', ub, ...
    'maps', maps, ...
    'problemInfo', problemInfo);
result.solution = struct( ...
    'x', x, ...
    'fval', fval, ...
    'exitflag', exitflag, ...
    'output', output);
result.decoded = decoded;
result.outputPaths = outputPaths;
end

function print_filter_report(report)
fprintf('Input filter report:\n');
fprintf('  original waste sources: %d\n', report.originalSourceCount);
fprintf('  kept waste sources: %d\n', report.keptSourceCount);
fprintf('  removed without route: %d\n', report.removedNoRouteCount);
fprintf('  original waste amount: %.6g\n', report.originalWasteAmount);
fprintf('  kept waste amount: %.6g\n', report.keptWasteAmount);
fprintf('  plant capacity: %.6g\n', report.totalPlantCapacity);
fprintf('  transfer capacity: %.6g\n', report.totalTransferCapacity);
end
