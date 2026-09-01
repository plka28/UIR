function [f, intcon, A, b, Aeq, beq, lb, ub, maps, problemInfo] = build_problem(data)
%BUILD_PROBLEM Form intlinprog matrices for waste routing optimization.
% This function only builds the optimization problem. It does not call
% intlinprog and does not decode solver results.

validation = validate_input_data(data);

% Normalize source volumes, capacities and distance matrices for indexing.
waste = numeric_column(data.wasteSources, 'waste_amount');
transferCapacity = numeric_column(data.transferStations, 'capacity');
plantCapacity = numeric_column(data.plants, 'capacity');

costPT = data.matrices.sourceToTransfer.distances;
costPU = data.matrices.sourceToPlant.distances;
costTU = data.matrices.transferToPlant.distances;

nSources = numel(waste);
nTransfers = numel(transferCapacity);
nPlants = numel(plantCapacity);
maps = create_variable_maps(nSources, nTransfers, nPlants);
nVariables = maps.nVariables;
routeAvailability = build_route_availability(costPT, costPU, costTU);
require_routeable_sources(routeAvailability);

% Objective uses distances as transport costs, weighted by waste where needed.
f = build_objective(waste, costPT, costPU, costTU, routeAvailability, maps);

nInequalities = nTransfers + nPlants;
% Inequality constraints represent transfer-station and plant capacities.
[A, b] = build_inequality_constraints(waste, transferCapacity, plantCapacity, maps);

nEqualities = nSources + nTransfers;
% Equality constraints represent unique assignment and transfer flow balance.
[Aeq, beq] = build_equality_constraints(waste, maps);

% x_pt and x_pu are binary; y_tu remains continuous and nonnegative.
lb = zeros(nVariables, 1);
ub = build_upper_bounds(routeAvailability, maps);

intcon = [maps.xPtRange, maps.xPuRange];

% Fail before solve_problem if any dimensions or integer sets drift.
check_problem_dimensions(f, intcon, A, b, Aeq, beq, lb, ub, maps, ...
    nInequalities, nEqualities);

problemInfo = struct();
problemInfo.nSources = nSources;
problemInfo.nTransfers = nTransfers;
problemInfo.nPlants = nPlants;
problemInfo.nXptVariables = numel(maps.x_pt);
problemInfo.nXpuVariables = numel(maps.x_pu);
problemInfo.nYtuVariables = numel(maps.y_tu);
problemInfo.nVariables = nVariables;
problemInfo.nBinaryVariables = numel(intcon);
problemInfo.nContinuousVariables = numel(maps.yTuRange);
problemInfo.nDisabledSourceToTransferRoutes = nnz(~routeAvailability.sourceToTransfer);
problemInfo.nDisabledSourceToPlantRoutes = nnz(~routeAvailability.sourceToPlant);
problemInfo.nDisabledTransferToPlantRoutes = nnz(~routeAvailability.transferToPlant);
problemInfo.nInequalities = nInequalities;
problemInfo.nEqualities = nEqualities;
problemInfo.totalWaste = sum(waste);
problemInfo.totalTransferCapacity = sum(transferCapacity);
problemInfo.totalPlantCapacity = sum(plantCapacity);
problemInfo.validation = validation;

print_problem_info(problemInfo);
end

function check_problem_dimensions(f, intcon, A, b, Aeq, beq, lb, ub, maps, ...
    nInequalities, nEqualities)
nVariables = maps.nVariables;

if ~isequal(size(f), [nVariables, 1])
    error('build_problem:DimensionMismatch', ...
        'f must be %d-by-1, got %d-by-%d', nVariables, size(f, 1), size(f, 2));
end

if ~isequal(size(A), [nInequalities, nVariables])
    error('build_problem:DimensionMismatch', ...
        'A must be %d-by-%d, got %d-by-%d', ...
        nInequalities, nVariables, size(A, 1), size(A, 2));
end

if ~isequal(size(b), [nInequalities, 1])
    error('build_problem:DimensionMismatch', ...
        'b must be %d-by-1, got %d-by-%d', nInequalities, size(b, 1), size(b, 2));
end

if ~isequal(size(Aeq), [nEqualities, nVariables])
    error('build_problem:DimensionMismatch', ...
        'Aeq must be %d-by-%d, got %d-by-%d', ...
        nEqualities, nVariables, size(Aeq, 1), size(Aeq, 2));
end

if ~isequal(size(beq), [nEqualities, 1])
    error('build_problem:DimensionMismatch', ...
        'beq must be %d-by-1, got %d-by-%d', nEqualities, size(beq, 1), size(beq, 2));
end

if ~isequal(size(lb), [nVariables, 1])
    error('build_problem:DimensionMismatch', ...
        'lb must be %d-by-1, got %d-by-%d', nVariables, size(lb, 1), size(lb, 2));
end

if ~isequal(size(ub), [nVariables, 1])
    error('build_problem:DimensionMismatch', ...
        'ub must be %d-by-1, got %d-by-%d', nVariables, size(ub, 1), size(ub, 2));
end

binaryIndices = sort([maps.xPtRange, maps.xPuRange]);
if ~isequal(sort(intcon(:).'), binaryIndices)
    error('build_problem:InvalidIntcon', ...
        'intcon must contain only x_pt and x_pu variable indices');
end

if any(ismember(intcon, maps.yTuRange))
    error('build_problem:InvalidIntcon', ...
        'intcon must not contain y_tu continuous variable indices');
end
end

function print_problem_info(problemInfo)
fprintf('Optimization problem diagnostics:\n');
fprintf('  waste sources: %d\n', problemInfo.nSources);
fprintf('  transfer stations: %d\n', problemInfo.nTransfers);
fprintf('  plants: %d\n', problemInfo.nPlants);
fprintf('  x_pt variables: %d\n', problemInfo.nXptVariables);
fprintf('  x_pu variables: %d\n', problemInfo.nXpuVariables);
fprintf('  y_tu variables: %d\n', problemInfo.nYtuVariables);
fprintf('  total variables: %d\n', problemInfo.nVariables);
fprintf('  equality constraints: %d\n', problemInfo.nEqualities);
fprintf('  inequality constraints: %d\n', problemInfo.nInequalities);
fprintf('  disabled source-transfer routes: %d\n', problemInfo.nDisabledSourceToTransferRoutes);
fprintf('  disabled source-plant routes: %d\n', problemInfo.nDisabledSourceToPlantRoutes);
fprintf('  disabled transfer-plant routes: %d\n', problemInfo.nDisabledTransferToPlantRoutes);
end

function [A, b] = build_inequality_constraints(waste, transferCapacity, plantCapacity, maps)
nInequalities = maps.nTransfers + maps.nPlants;
maxNonzeros = maps.nTransfers * maps.nSources + ...
    maps.nPlants * maps.nSources + maps.nPlants * maps.nTransfers;
rowIndex = zeros(maxNonzeros, 1);
colIndex = zeros(maxNonzeros, 1);
values = zeros(maxNonzeros, 1);
entry = 0;

b = zeros(nInequalities, 1);

% Transfer capacity: sum_p waste(p) * x_pt(p,t) <= capacity_transfer(t).
for t = 1:maps.nTransfers
    row = t;
    for p = 1:maps.nSources
        entry = entry + 1;
        rowIndex(entry) = row;
        colIndex(entry) = maps.x_pt(p, t);
        values(entry) = waste(p);
    end
    b(row) = transferCapacity(t);
end

% Plant capacity: direct waste plus transfer flows must not exceed capacity.
for u = 1:maps.nPlants
    row = maps.nTransfers + u;
    for p = 1:maps.nSources
        entry = entry + 1;
        rowIndex(entry) = row;
        colIndex(entry) = maps.x_pu(p, u);
        values(entry) = waste(p);
    end
    for t = 1:maps.nTransfers
        entry = entry + 1;
        rowIndex(entry) = row;
        colIndex(entry) = maps.y_tu(t, u);
        values(entry) = 1;
    end
    b(row) = plantCapacity(u);
end

A = sparse(rowIndex(1:entry), colIndex(1:entry), values(1:entry), ...
    nInequalities, maps.nVariables);
end

function [Aeq, beq] = build_equality_constraints(waste, maps)
nEqualities = maps.nSources + maps.nTransfers;
maxNonzeros = maps.nSources * (maps.nTransfers + maps.nPlants) + ...
    maps.nTransfers * (maps.nPlants + maps.nSources);
rowIndex = zeros(maxNonzeros, 1);
colIndex = zeros(maxNonzeros, 1);
values = zeros(maxNonzeros, 1);
entry = 0;

beq = zeros(nEqualities, 1);

% Unique assignment: sum_t x_pt(p,t) + sum_u x_pu(p,u) = 1.
for p = 1:maps.nSources
    row = p;
    for t = 1:maps.nTransfers
        entry = entry + 1;
        rowIndex(entry) = row;
        colIndex(entry) = maps.x_pt(p, t);
        values(entry) = 1;
    end
    for u = 1:maps.nPlants
        entry = entry + 1;
        rowIndex(entry) = row;
        colIndex(entry) = maps.x_pu(p, u);
        values(entry) = 1;
    end
    beq(row) = 1;
end

% Transfer balance: sum_u y_tu(t,u) = sum_p waste(p) * x_pt(p,t).
for t = 1:maps.nTransfers
    row = maps.nSources + t;
    for u = 1:maps.nPlants
        entry = entry + 1;
        rowIndex(entry) = row;
        colIndex(entry) = maps.y_tu(t, u);
        values(entry) = 1;
    end
    for p = 1:maps.nSources
        entry = entry + 1;
        rowIndex(entry) = row;
        colIndex(entry) = maps.x_pt(p, t);
        values(entry) = -waste(p);
    end
end

Aeq = sparse(rowIndex(1:entry), colIndex(1:entry), values(1:entry), ...
    nEqualities, maps.nVariables);
end

function f = build_objective(waste, costPT, costPU, costTU, routeAvailability, maps)
f = zeros(maps.nVariables, 1);

for p = 1:maps.nSources
    for t = 1:maps.nTransfers
        if routeAvailability.sourceToTransfer(p, t)
            f(maps.x_pt(p, t)) = waste(p) * costPT(p, t);
        end
    end
end

for p = 1:maps.nSources
    for u = 1:maps.nPlants
        if routeAvailability.sourceToPlant(p, u)
            f(maps.x_pu(p, u)) = waste(p) * costPU(p, u);
        end
    end
end

for t = 1:maps.nTransfers
    for u = 1:maps.nPlants
        if routeAvailability.transferToPlant(t, u)
            f(maps.y_tu(t, u)) = costTU(t, u);
        end
    end
end
end

function routeAvailability = build_route_availability(costPT, costPU, costTU)
routeAvailability = struct();
routeAvailability.transferToPlant = isfinite(costTU);
transferCanReachPlant = any(routeAvailability.transferToPlant, 2).';
routeAvailability.sourceToTransfer = isfinite(costPT) & ...
    repmat(transferCanReachPlant, size(costPT, 1), 1);
routeAvailability.sourceToPlant = isfinite(costPU);
end

function require_routeable_sources(routeAvailability)
sourceHasRoute = any(routeAvailability.sourceToTransfer, 2) | ...
    any(routeAvailability.sourceToPlant, 2);
if any(~sourceHasRoute)
    error('build_problem:UnrouteableSources', ...
        '%d waste sources have no available route after filtering', nnz(~sourceHasRoute));
end
end

function ub = build_upper_bounds(routeAvailability, maps)
ub = zeros(maps.nVariables, 1);

for p = 1:maps.nSources
    for t = 1:maps.nTransfers
        if routeAvailability.sourceToTransfer(p, t)
            ub(maps.x_pt(p, t)) = 1;
        end
    end
end

for p = 1:maps.nSources
    for u = 1:maps.nPlants
        if routeAvailability.sourceToPlant(p, u)
            ub(maps.x_pu(p, u)) = 1;
        end
    end
end

for t = 1:maps.nTransfers
    for u = 1:maps.nPlants
        if routeAvailability.transferToPlant(t, u)
            ub(maps.y_tu(t, u)) = inf;
        end
    end
end
end

function values = numeric_column(tableData, columnName)
rawValues = tableData.(columnName);
if isnumeric(rawValues)
    values = double(rawValues);
else
    values = str2double(string(rawValues));
end
values = values(:);
end
