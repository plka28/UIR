function decoded = decode_solution(x, data, maps, problemInfo, fval)
%DECODE_SOLUTION Convert intlinprog vector back to domain tables.

if nargin < 5
    fval = NaN;
end

x = x(:);
if numel(x) ~= maps.nVariables
    error('decode_solution:InvalidSolutionSize', ...
        'x has %d elements, expected %d', numel(x), maps.nVariables);
end

% Reconstruct matrix-shaped variables using the index maps from build_problem.
Xpt = zeros(maps.nSources, maps.nTransfers);
Xpu = zeros(maps.nSources, maps.nPlants);
Ytu = zeros(maps.nTransfers, maps.nPlants);

for p = 1:maps.nSources
    for t = 1:maps.nTransfers
        Xpt(p, t) = x(maps.x_pt(p, t));
    end
end

for p = 1:maps.nSources
    for u = 1:maps.nPlants
        Xpu(p, u) = x(maps.x_pu(p, u));
    end
end

for t = 1:maps.nTransfers
    for u = 1:maps.nPlants
        Ytu(t, u) = x(maps.y_tu(t, u));
    end
end

binaryTolerance = 1e-6;
XptRounded = round_binary_matrix(Xpt, binaryTolerance);
XpuRounded = round_binary_matrix(Xpu, binaryTolerance);

% Build output tables for source assignments, transfer flows and summary metrics.
sourceAssignments = build_source_assignments(data, XptRounded, XpuRounded);
transferFlows = build_transfer_flows(data, Ytu);
summary = build_summary_table(sourceAssignments, transferFlows, problemInfo, fval);

decoded = struct();
decoded.Xpt = XptRounded;
decoded.Xpu = XpuRounded;
decoded.Ytu = Ytu;
decoded.rawXpt = Xpt;
decoded.rawXpu = Xpu;
decoded.sourceAssignments = sourceAssignments;
decoded.transferFlows = transferFlows;
decoded.summary = summary;
end

function rounded = round_binary_matrix(values, tolerance)
rounded = values;
rounded(abs(values) <= tolerance) = 0;
rounded(abs(values - 1) <= tolerance) = 1;
rounded = round(rounded);
end

function assignments = build_source_assignments(data, Xpt, Xpu)
% Each source is decoded to either a transfer station or a direct plant route.
sourceIds = table_ids(data.wasteSources);
transferIds = table_ids(data.transferStations);
plantIds = table_ids(data.plants);
transferNames = table_names(data.transferStations);
plantNames = table_names(data.plants);
waste = numeric_column(data.wasteSources, 'waste_amount');

nSources = numel(sourceIds);
assignmentType = strings(nSources, 1);
destinationId = strings(nSources, 1);
destinationName = strings(nSources, 1);
distanceKm = nan(nSources, 1);
transportCost = nan(nSources, 1);

for p = 1:nSources
    transferIndex = find(Xpt(p, :) == 1, 1);
    plantIndex = find(Xpu(p, :) == 1, 1);

    if ~isempty(transferIndex)
        assignmentType(p) = "transfer_station";
        destinationId(p) = transferIds(transferIndex);
        destinationName(p) = transferNames(transferIndex);
        distanceKm(p) = data.matrices.sourceToTransfer.distances(p, transferIndex);
    elseif ~isempty(plantIndex)
        assignmentType(p) = "plant";
        destinationId(p) = plantIds(plantIndex);
        destinationName(p) = plantNames(plantIndex);
        distanceKm(p) = data.matrices.sourceToPlant.distances(p, plantIndex);
    else
        assignmentType(p) = "unassigned";
        destinationId(p) = "";
        destinationName(p) = "";
    end

    if ~isnan(distanceKm(p))
        transportCost(p) = waste(p) * distanceKm(p);
    end
end

assignments = table(sourceIds, assignmentType, destinationId, destinationName, ...
    waste, distanceKm, transportCost, ...
    'VariableNames', {'source_id', 'assignment_type', 'destination_id', ...
    'destination_name', 'waste_amount', 'distance_km', 'transport_cost'});
end

function flows = build_transfer_flows(data, Ytu)
% Export only positive transfer-to-plant flows.
transferIds = table_ids(data.transferStations);
plantIds = table_ids(data.plants);
transferNames = table_names(data.transferStations);
plantNames = table_names(data.plants);

flowTolerance = 1e-9;
rows = find(Ytu > flowTolerance);
nFlows = numel(rows);

transferStationId = strings(nFlows, 1);
transferStationName = strings(nFlows, 1);
plantId = strings(nFlows, 1);
plantName = strings(nFlows, 1);
wasteAmount = zeros(nFlows, 1);
distanceKm = zeros(nFlows, 1);
transportCost = zeros(nFlows, 1);

for i = 1:nFlows
    [t, u] = ind2sub(size(Ytu), rows(i));
    transferStationId(i) = transferIds(t);
    transferStationName(i) = transferNames(t);
    plantId(i) = plantIds(u);
    plantName(i) = plantNames(u);
    wasteAmount(i) = Ytu(t, u);
    distanceKm(i) = data.matrices.transferToPlant.distances(t, u);
    transportCost(i) = wasteAmount(i) * distanceKm(i);
end

flows = table(transferStationId, transferStationName, plantId, plantName, ...
    wasteAmount, distanceKm, transportCost, ...
    'VariableNames', {'transfer_station_id', 'transfer_station_name', ...
    'plant_id', 'plant_name', 'waste_amount', 'distance_km', 'transport_cost'});
end

function summary = build_summary_table(assignments, flows, problemInfo, fval)
% One-row-equivalent metric table for quick checks and downstream reports.
directMask = assignments.assignment_type == "plant";
transferMask = assignments.assignment_type == "transfer_station";
totalSourceCost = sum_without_nan(assignments.transport_cost);
totalTransferCost = sum_without_nan(flows.transport_cost);
totalCost = totalSourceCost + totalTransferCost;

metric = [
    "objective_value";
    "decoded_total_cost";
    "source_leg_cost";
    "transfer_to_plant_cost";
    "waste_sources";
    "transfer_stations";
    "plants";
    "x_pt_variables";
    "x_pu_variables";
    "y_tu_variables";
    "total_variables";
    "assigned_to_transfer_count";
    "assigned_direct_to_plant_count";
    "transfer_flow_count";
    "total_waste_amount";
    "total_transfer_capacity";
    "total_plant_capacity"
];

value = [
    string(fval);
    string(totalCost);
    string(totalSourceCost);
    string(totalTransferCost);
    string(problemInfo.nSources);
    string(problemInfo.nTransfers);
    string(problemInfo.nPlants);
    string(problemInfo.nXptVariables);
    string(problemInfo.nXpuVariables);
    string(problemInfo.nYtuVariables);
    string(problemInfo.nVariables);
    string(sum(transferMask));
    string(sum(directMask));
    string(height(flows));
    string(problemInfo.totalWaste);
    string(problemInfo.totalTransferCapacity);
    string(problemInfo.totalPlantCapacity)
];

summary = table(metric, value);
end

function ids = table_ids(tableData)
ids = strtrim(string(tableData.id));
ids = ids(:);
end

function names = table_names(tableData)
names = strtrim(string(tableData.name));
names = names(:);
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

function total = sum_without_nan(values)
total = sum(values(~isnan(values)));
end
