function maps = create_variable_maps(nSources, nTransfers, nPlants)
%CREATE_VARIABLE_MAPS Assign vector indices to optimization variables.
% Variables are stored in one column vector for intlinprog:
%   x_pt(p,t) - binary source-to-transfer assignment;
%   x_pu(p,u) - binary direct source-to-plant assignment;
%   y_tu(t,u) - continuous transfer-to-plant flow.

validate_count(nSources, 'nSources');
validate_count(nTransfers, 'nTransfers');
validate_count(nPlants, 'nPlants');

maps = struct();
maps.nSources = nSources;
maps.nTransfers = nTransfers;
maps.nPlants = nPlants;

maps.x_pt = zeros(nSources, nTransfers);
maps.x_pu = zeros(nSources, nPlants);
maps.y_tu = zeros(nTransfers, nPlants);

index = 0;
for p = 1:nSources
    for t = 1:nTransfers
        index = index + 1;
        maps.x_pt(p, t) = index;
    end
end
maps.xPtRange = 1:index;

xPuStart = index + 1;
for p = 1:nSources
    for u = 1:nPlants
        index = index + 1;
        maps.x_pu(p, u) = index;
    end
end
maps.xPuRange = xPuStart:index;

yTuStart = index + 1;
for t = 1:nTransfers
    for u = 1:nPlants
        index = index + 1;
        maps.y_tu(t, u) = index;
    end
end
maps.yTuRange = yTuStart:index;

maps.nVariables = index;
maps.nBinaryVariables = numel(maps.x_pt) + numel(maps.x_pu);
maps.nContinuousVariables = numel(maps.y_tu);
end

function validate_count(value, name)
if ~isscalar(value) || value < 1 || fix(value) ~= value
    error('create_variable_maps:InvalidCount', ...
        '%s must be a positive integer', name);
end
end
