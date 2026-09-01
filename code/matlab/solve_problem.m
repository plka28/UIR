function [x, fval, exitflag, output] = solve_problem(f, intcon, A, b, Aeq, beq, lb, ub)
%SOLVE_PROBLEM Run intlinprog for the prepared waste routing problem.

% Validate compatibility with arrays produced by build_problem.
validate_solver_inputs(f, intcon, A, b, Aeq, beq, lb, ub);

% HiGHS in MATLAB R2024a can crash during presolve on this large binary
% assignment model. The legacy algorithm is slower, but avoids highs.dll.
options = optimoptions('intlinprog', ...
    'Algorithm', 'legacy', ...
    'Display', 'iter', ...
    'RelativeGapTolerance', 1e-4, ...
    'MaxTime', 3600);

% Measure solver time only; decoding and CSV saving happen in run_optimization.
startTime = tic;
[x, fval, exitflag, output] = intlinprog(f, intcon, A, b, Aeq, beq, lb, ub, options);
output.solveTimeSeconds = toc(startTime);
output.solverAlgorithm = string(options.Algorithm);
end

function validate_solver_inputs(f, intcon, A, b, Aeq, beq, lb, ub)
nVariables = numel(f);

if ~iscolumn(f)
    error('solve_problem:InvalidInput', 'f must be a column vector');
end

if ~isempty(A) && size(A, 2) ~= nVariables
    error('solve_problem:InvalidInput', ...
        'A must have %d columns, got %d', nVariables, size(A, 2));
end

if ~isempty(Aeq) && size(Aeq, 2) ~= nVariables
    error('solve_problem:InvalidInput', ...
        'Aeq must have %d columns, got %d', nVariables, size(Aeq, 2));
end

if ~isempty(A) && numel(b) ~= size(A, 1)
    error('solve_problem:InvalidInput', ...
        'b length must match A row count');
end

if ~isempty(Aeq) && numel(beq) ~= size(Aeq, 1)
    error('solve_problem:InvalidInput', ...
        'beq length must match Aeq row count');
end

if numel(lb) ~= nVariables || numel(ub) ~= nVariables
    error('solve_problem:InvalidInput', ...
        'lb and ub must match f length');
end

if any(intcon < 1) || any(intcon > nVariables) || any(fix(intcon) ~= intcon)
    error('solve_problem:InvalidInput', ...
        'intcon must contain valid integer variable indices');
end
end
