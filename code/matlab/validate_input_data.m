function report = validate_input_data(data)
%VALIDATE_INPUT_DATA Check prepared tables and distance matrix dimensions.

% Required columns define the input contract for the MATLAB model.
requiredWasteColumns = ["id", "service_zone", "address", "district", ...
    "waste_type", "container_type", "container_volume_m3", ...
    "container_count", "waste_amount", "latitude", "longitude"];
requiredFacilityColumns = ["id", "name", "capacity", "latitude", "longitude"];

require_columns(data.wasteSources, requiredWasteColumns, 'waste_sources.csv');
require_columns(data.transferStations, requiredFacilityColumns, 'transfer_stations.csv');
require_columns(data.plants, requiredFacilityColumns, 'plants.csv');

% Waste volumes and facility capacities must be numeric and nonnegative.
wasteAmounts = numeric_column(data.wasteSources, 'waste_amount', 'waste_sources.csv');
transferCapacities = numeric_column(data.transferStations, 'capacity', 'transfer_stations.csv');
plantCapacities = numeric_column(data.plants, 'capacity', 'plants.csv');

assert_nonnegative(wasteAmounts, 'waste_sources.csv', 'waste_amount');
assert_nonnegative(transferCapacities, 'transfer_stations.csv', 'capacity');
assert_nonnegative(plantCapacities, 'plants.csv', 'capacity');

wasteIds = table_ids(data.wasteSources);
transferIds = table_ids(data.transferStations);
plantIds = table_ids(data.plants);

% Matrix dimensions and ids must match the prepared table ordering exactly.
validate_matrix(data.matrices.sourceToTransfer, wasteIds, transferIds, 'source_to_transfer.csv');
validate_matrix(data.matrices.sourceToPlant, wasteIds, plantIds, 'source_to_plant.csv');
validate_matrix(data.matrices.transferToPlant, transferIds, plantIds, 'transfer_to_plant.csv');

totalWaste = sum_without_nan(wasteAmounts);
totalTransferCapacity = sum_without_nan(transferCapacities);
totalPlantCapacity = sum_without_nan(plantCapacities);

report = struct();
report.totalWasteAmount = totalWaste;
report.totalTransferCapacity = totalTransferCapacity;
report.totalPlantCapacity = totalPlantCapacity;
report.warnings = strings(0, 1);

% Capacity warnings flag likely infeasible data without blocking direct routing.
if totalPlantCapacity < totalWaste
    report.warnings(end + 1, 1) = sprintf([ ...
        'Total plant capacity %.6g is lower than total waste amount %.6g. ' ...
        'Check units or input data before building the optimization model.'], ...
        totalPlantCapacity, totalWaste);
end

if totalTransferCapacity < totalWaste
    report.warnings(end + 1, 1) = sprintf([ ...
        'Total transfer station capacity %.6g is lower than total waste amount %.6g. ' ...
        'This can be valid only if part of waste is transported directly to plants.'], ...
        totalTransferCapacity, totalWaste);
end
end

function require_columns(tableData, requiredColumns, tableName)
actualColumns = string(tableData.Properties.VariableNames);
missingColumns = setdiff(requiredColumns, actualColumns, 'stable');
if ~isempty(missingColumns)
    error('validate_input_data:MissingColumns', ...
        '%s is missing required columns: %s', tableName, strjoin(missingColumns, ', '));
end
end

function values = numeric_column(tableData, columnName, tableName)
rawValues = tableData.(columnName);
if isnumeric(rawValues)
    values = double(rawValues);
else
    values = str2double(string(rawValues));
end

if any(isnan(values))
    error('validate_input_data:InvalidNumericColumn', ...
        '%s column %s contains non-numeric values', tableName, columnName);
end
end

function assert_nonnegative(values, tableName, columnName)
if any(values < 0)
    error('validate_input_data:NegativeValues', ...
        '%s column %s contains negative values', tableName, columnName);
end
end

function ids = table_ids(tableData)
ids = strtrim(string(tableData.id));
ids = ids(:);
end

function validate_matrix(matrixData, expectedSourceIds, expectedTargetIds, matrixName)
actualRows = size(matrixData.distances, 1);
actualCols = size(matrixData.distances, 2);

if actualRows ~= numel(expectedSourceIds)
    error('validate_input_data:MatrixRowMismatch', ...
        '%s has %d rows, expected %d', matrixName, actualRows, numel(expectedSourceIds));
end

if actualCols ~= numel(expectedTargetIds)
    error('validate_input_data:MatrixColumnMismatch', ...
        '%s has %d columns, expected %d', matrixName, actualCols, numel(expectedTargetIds));
end

if ~isequal(matrixData.sourceIds(:), expectedSourceIds(:))
    error('validate_input_data:MatrixSourceIdsMismatch', ...
        '%s source ids do not match prepared source table order', matrixName);
end

if ~isequal(matrixData.targetIds(:), expectedTargetIds(:))
    error('validate_input_data:MatrixTargetIdsMismatch', ...
        '%s target ids do not match prepared target table order', matrixName);
end

finiteDistances = matrixData.distances(~isnan(matrixData.distances));
if any(finiteDistances < 0)
    error('validate_input_data:NegativeDistances', ...
        '%s contains negative distance values', matrixName);
end
end

function total = sum_without_nan(values)
total = sum(values(~isnan(values)));
end
