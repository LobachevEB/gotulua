DB = DBOpen("./Ts.db")

-- After the project code is selected in the Task table, we write the Project id to the Task table.
-- The first parameter in the Lookup functions is the table that we looked up in; the second parameter - the destination table
OnLookupProject = function(pProject, pTask)
    pTask.ProjectId = pProject.id
    if pTask.id > 0 then
        pTask:Update()
    else
        -- If the record id in destination table is equal to 0 this means that the current record is not inserted to the table. Let's insert it.
        pTask:Insert() 
    end
end

-- After the Project code is selected in the Timesheet table, we write the Project id to the Task table
OnLookupProjectForTs = function(pProject, pTs)
    pTs.ProjectId = pProject.id
    pTs.ProjectName = pProject.Name
    -- If the Update() method return false most likely this means that the current record is not inserted to the table. Let's insert it.
    if not pTs:Update() then
        pTs:Insert()
    end
end

-- After the Task code is selected in the Timesheet table, we write the Project id to the Task table
OnLookupTaskForTs = function(pTask, pTs)
    pTs.TaskId = pTask.id
    pTs.TaskNumber = pTask.Number
    if not pTs:Update() then
        pTs:Insert()
    end
end

-- The Browse object may contain fields of two types:
-- Table fields. Regular table fields that can be edited or filtered.
-- Function fields. Calculated fields that can not be edited or filtered.
-- Following functions - GetProjectName, GetProjectNameForTs, GetTaskNumber - are the functions that called for
-- the Function-type fields for each table row that's shown by the Browse.
-- These functions get the only one parameter: the current table record.
-- These function should return the value.
GetProjectName = function(pTask)
    local proj = DBOpenTable(DB, "Project")
    if proj:FindByID(pTask.ProjectId) then
        return proj.Name
    else
        return "Project name"
    end
end

GetProjectNameForTs = function(pTs)
    local proj = DBOpenTable(DB, "Project")
    if proj:FindByID(pTs.ProjectId) then
        return proj.Name
    else
        return "Project name"
    end
end

GetTaskNumber = function(pTs)
    local task = DBOpenTable(DB, "Task")
    if task:FindByID(pTs.TaskId) then
        return task.Number
    else
        return "Task number"
    end
end

-- The OnAfterInsert functions are call after current record is inserted to the table.
-- Only one parameter is passed: the record that was inserted
TSOnAfterInsert = function(ins)
    if ins.Date == '' then
        ins.Date = Date()
    end
    if ins.StartTime == '' then
        ins.StartTime = Time()
    end
    ins:Update()
end

-- The OnAfterUpdate functions are call after current record was updated in the table.
-- Two parameters are passed: the record that was updated and the before update record's copy (xRec)
TSOnAfterUpdate = function(rec, xRec)
    if rec.StartTime ~= '' and rec.StopTime ~= '' then
        rec.Duration = TimeDiff(rec.StartTime, rec.StopTime, 'm')
        rec:Update()
    end
end

TSPrint = function(ts)
    if ts:Find() then
        local prj = DBOpenTable(DB, "Project")
        local FL = io.open(".\\ts.csv", "w")
        local n = 0
        repeat
            FL:write(string.format("%s;%s;%s;%d\n", ts.ProjectName, ts.TaskNumber, ts.Date, ts.Duration))
            FL:flush()
            n = n + 1
        until not ts:Next()
        FL:close()
        Message(string.format("Выгружено %d строк", n))
    else
        Message("Нет данных!")
    end
end


local ts = DBOpenTable(DB, "Sheet")
ts:SetOnAfterInsert('TSOnAfterInsert')
ts:SetOnAfterUpdate('TSOnAfterUpdate')

local prj = DBOpenTable(DB, "Project")
local task = DBOpenTable(DB, "Task")

-- The AddBrowse function create the Browse object, i.e. the table that will be shown in the screen
-- The first parameter - the table variable, that was returned by DBOpenTable function
-- The second parameter - the current Browser title
local bPrj = AddBrowse(prj, "Projects")
-- AddField method get the structured string parameter.
-- For Table fields: "n::Field1 name;c::Field1 Caption;e::Is Field1 editable or not|..."
-- For Function fields: "n::Field1 name;c::Field1 Caption;f::Name of function that will be called for this field"
bPrj:AddField("n::Name;c::Project Name;e::true|"..
"n::Description;c::Project Description;e::true")

-- The AddLookup function create the Lookup Browse object, i.e. the table that will be shown in the screen
-- when we press the Enter key in appropriate field in the main Browse object.
-- The first parameter - the table variable, that was returned by DBOpenTable function
-- The second parameter - the current Browser title
local lPrj = AddLookup(prj, "Choose Project")
lPrj:AddField("n::Name;c::Project Name;e::false")

local lTask = AddLookup(task, "Choose Task")
lTask:AddField("n::Number;c::Task Number;e::false")

local bTask = AddBrowse(task, "Tasks")
bTask:AddField(
"n::Number;c::Task Number;e::true|"..
"n::Project;c::Project Name;f::GetProjectName|".. -- This is the Function field
"n::Description;c::Task Description;e::true|"..
"n::Closed;c::Closed;e::true")
-- SetFieldLookup link the lookup Browse lPrj to the exact field in the main Browse bTask
-- Parameters:
-- Name of the Field that the lookup is linked to. When we press the Enter key in this field, the lookup table will be shown.
-- The lookup object created by AddLookup function
-- The name of the function that will be called after we choose the record in lookup table and press Enter
bTask:SetFieldLookup("Project", lPrj, "OnLookupProject")

local bTs = AddBrowse(ts, "Timesheet")
local fld = "n::ProjectName;c::Project Name;e::true|" ..
    "n::TaskNumber;c::Task Number;e::true|" ..
    "n::Date;c::Date;e::true|" ..
    "n::StartTime;c::Job Start Time;e::true|" ..
    "n::StopTime;c::Job Stop Time;e::true|" ..
    "n::Duration;c::Duration, min;e::true|" ..
    "n::Description;c::Description;e::true"
bTs:AddField(fld)
bTs:SetFieldLookup("ProjectName", lPrj, "OnLookupProjectForTs")
bTs:SetFieldLookup("TaskNumber", lTask, "OnLookupTaskForTs")

-- The AddButton method adds the button in the bottom part of the current Browse
-- Parameters:
-- Button caption
-- Name of the function that called when we click mouse on the button. The parameter passed to this function is the table on that current Browse object is based.
bTs:AddButton("Save to File", "TSPrint")

bTs:Show()
bPrj:Show()
bTask:Show()









