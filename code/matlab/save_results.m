function outputPaths = save_results(decoded, outputDir)
%SAVE_RESULTS Write decoded optimization results to CSV files.

% Default output location is code/output/optimization relative to this file.
if nargin < 2 || strlength(string(outputDir)) == 0
    matlabDir = fileparts(mfilename('fullpath'));
    outputDir = fullfile(matlabDir, '..', 'output', 'optimization');
end

% Create the output directory for a fresh project checkout.
if exist(outputDir, 'dir') ~= 7
    mkdir(outputDir);
end

outputPaths = struct();
outputPaths.sourceAssignments = fullfile(outputDir, 'source_assignments.csv');
outputPaths.transferFlows = fullfile(outputDir, 'transfer_flows.csv');
outputPaths.summary = fullfile(outputDir, 'summary.csv');

% Persist the decoded domain tables; no QGIS export is done here.
writetable(decoded.sourceAssignments, outputPaths.sourceAssignments);
writetable(decoded.transferFlows, outputPaths.transferFlows);
writetable(decoded.summary, outputPaths.summary);
end
