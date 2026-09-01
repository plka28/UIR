function [filteredData, report] = filter_optimization_input(data)
%FILTER_OPTIMIZATION_INPUT Drop sources that cannot be used in the MILP.
% The raw route matrices may contain NaN values from Valhalla for directions
% that cannot be routed. This function keeps only sources with at least one
% usable direct or transfer route. Capacity feasibility is left to the
% optimization model and to validate_input_data warnings.

validate_input_data(data);

filteredData = data;
waste = numeric_column(data.wasteSources, 'waste_amount');
plantCapacity = numeric_column(data.plants, 'capacity');
transferCapacity = numeric_column(data.transferStations, 'capacity');

costPT = data.matrices.sourceToTransfer.distances;
costPU = data.matrices.sourceToPlant.distances;
costTU = data.matrices.transferToPlant.distances;

sourceHasRoute = routeable_sources(costPT, costPU, costTU);
keepMask = sourceHasRoute;
keptIndices = find(keepMask);

filteredData.wasteSources = data.wasteSources(keepMask, :);
filteredData.matrices.sourceToTransfer = filter_source_matrix( ...
    data.matrices.sourceToTransfer, keepMask);
filteredData.matrices.sourceToPlant = filter_source_matrix( ...
    data.matrices.sourceToPlant, keepMask);

report = struct();
report.originalSourceCount = height(data.wasteSources);
report.keptSourceCount = numel(keptIndices);
report.removedNoRouteCount = report.originalSourceCount - report.keptSourceCount;
report.originalWasteAmount = sum(waste);
report.keptWasteAmount = sum(waste(keptIndices));
report.totalPlantCapacity = sum(plantCapacity);
report.totalTransferCapacity = sum(transferCapacity);
report.removedNoRouteSourceIds = source_ids(data.wasteSources, ~sourceHasRoute);

if report.keptSourceCount == 0
    error('filter_optimization_input:NoSourcesLeft', ...
        'No waste sources remain after route filtering');
end

filteredData.filterReport = report;
end

function mask = routeable_sources(costPT, costPU, costTU)
directAvailable = isfinite(costPU);

% A source-to-transfer edge is useful only if that transfer can reach a plant.
transferCanReachPlant = any(isfinite(costTU), 2).';
viaTransferAvailable = isfinite(costPT) & repmat(transferCanReachPlant, size(costPT, 1), 1);

mask = any(directAvailable, 2) | any(viaTransferAvailable, 2);
end

function matrixData = filter_source_matrix(matrixData, keepMask)
matrixData.sourceIds = matrixData.sourceIds(keepMask);
matrixData.distances = matrixData.distances(keepMask, :);
end

function ids = source_ids(wasteSources, mask)
ids = string(wasteSources.id);
ids = ids(mask);
ids = ids(:);
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
