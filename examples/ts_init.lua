-- Here we initialize the SQLite database
DB = DBCreate("./Ts.db")

-- Parameters of DBCreateTable are:
-- 1. DB object
-- 2. Table structure in format "n::Name of Field1;t::Type of Field1;l::Length of Field1 (if applicable)|n::Name of Field 2..." 
-- 3. The flag that say what to do if the table is already exists in DB. True - open the table, false - show the error message.
p = DBCreateTable(DB,"Project","n::Name;t::Text;l::100|"..
                               "n::Description;t::Text",true)
t = DBCreateTable(DB,"Task","n::ProjectId;t::Integer|"..
                            "n::Number;t::Text;l::50|"..
                            "n::Description;t::Text|"..
                            "n::Closed;t::Boolean",true)
ts = DBCreateTable(DB,"Sheet","n::ProjectId;t::Integer|"..
                              "n::ProjectName;t::Text;l::100|"..
                              "n::TaskId;t::Integer|"..
                              "n::TaskNumber;t::Text;l::50|"..
                              "n::Date;t::Date|"..
                              "n::StartTime;t::Time|"..
                              "n::StopTime;t::Time|"..
                              "n::Duration;t::Float|"..
                              "n::Description;t::Text",true)

DBClose(DB)
Message("Done!")





















