function data = load_input_data()
%LOAD_INPUT_DATA Read prepared CSV files and distance matrices.

% Resolve paths from this file so execution works from code/matlab.
matlabDir = fileparts(mfilename('fullpath'));
projectDir = fullfile(matlabDir, '..');

paths = struct();
paths.wasteSources = fullfile(projectDir, 'data', 'prepared', 'waste_sources.csv');
paths.transferStations = fullfile(projectDir, 'data', 'prepared', 'transfer_stations.csv');
paths.plants = fullfile(projectDir, 'data', 'prepared', 'plants.csv');
paths.sourceToTransfer = fullfile(projectDir, 'output', 'matrices', 'source_to_transfer.csv');
paths.sourceToPlant = fullfile(projectDir, 'output', 'matrices', 'source_to_plant.csv');
paths.transferToPlant = fullfile(projectDir, 'output', 'matrices', 'transfer_to_plant.csv');

data = struct();
data.paths = paths;
data.wasteSources = read_required_table(paths.wasteSources);
data.transferStations = read_required_table(paths.transferStations);
data.plants = read_required_table(paths.plants);

% Distance matrices keep row ids, column ids and numeric values together.
data.matrices = struct();
data.matrices.sourceToTransfer = read_distance_matrix(paths.sourceToTransfer);
data.matrices.sourceToPlant = read_distance_matrix(paths.sourceToPlant);
data.matrices.transferToPlant = read_distance_matrix(paths.transferToPlant);
end

function tableData = read_required_table(filePath)
assert_file_exists(filePath);
tableData = readtable(filePath);
end

function matrixData = read_distance_matrix(filePath)
assert_file_exists(filePath);

% Matrix CSV format: first row is target ids, first column is source ids.
raw = readcell(filePath);
if size(raw, 1) < 2 || size(raw, 2) < 2
    error('load_input_data:InvalidMatrixFile', ...
        'Matrix file must contain a header row, source ids and at least one value: %s', filePath);
end

targetIds = cell_to_string_vector(raw(1, 2:end));
sourceIds = cell_to_string_vector(raw(2:end, 1));
values = raw(2:end, 2:end);

distances = nan(size(values));
for row = 1:size(values, 1)
    for col = 1:size(values, 2)
        distances(row, col) = cell_to_number(values{row, col});
    end
end

matrixData = struct();
matrixData.path = filePath;
matrixData.sourceIds = sourceIds(:);
matrixData.targetIds = targetIds(:).';
matrixData.distances = distances;
end

function assert_file_exists(filePath)
if exist(filePath, 'file') ~= 2
    error('load_input_data:MissingFile', ...
        'Required input file does not exist: %s', filePath);
end
end

function values = cell_to_string_vector(cells)
values = strings(size(cells));
for i = 1:numel(cells)
    values(i) = cell_to_string(cells{i});
end
end

function value = cell_to_string(cellValue)
if isempty(cellValue)
    value = "";
elseif isnumeric(cellValue) || islogical(cellValue)
    value = string(cellValue);
else
    value = string(cellValue);
end
value = strtrim(value);
end

function value = cell_to_number(cellValue)
if isempty(cellValue)
    value = NaN;
elseif isnumeric(cellValue)
    value = double(cellValue);
elseif islogical(cellValue)
    value = double(cellValue);
else
    textValue = strtrim(string(cellValue));
    if textValue == "" || strcmpi(textValue, "NaN")
        value = NaN;
    else
        value = str2double(strrep(textValue, ',', '.'));
    end
end
end
