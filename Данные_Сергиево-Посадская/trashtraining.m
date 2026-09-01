%% ЗАДАЧА О НАЗНАЧЕНИИ ПОМОЙКА-ПЕРЕВАЛКА ИЛИ ПОМОЙКА-ЗАВОД. ЗАВОДЫ С БЕСКОНЕЧНОЙ МОЩНОСТЬЮ


%% costPT - матрица стоимости привязки помойка-перевалка (строки-помойки, столбцы-перевалки)
%% costPU - матрица стоимости привязки помойка-завол (строки-помойки, столбцы-заводы)
%% costTU - вектор стоимости перевозки перевалка-ближайщий-к-ней-завод (вектор СТОЛБЕЦ)
%% MP - вектор стоимости перевозки помойка-перевалка  (вектор СТОЛБЕЦ)
%% MT - вектор стоимости перевозки перевалка-ближайщий-к-ней-завод (вектор СТОЛБЕЦ)


%% random data simulation
NP=1000;%число помоек
NT=10;%число перевалок
NU=5;%число заводов
costPT=rand(NP,NT);
costPU=rand(NP,NU);
costTU=rand(NT,1)/2;
MP=rand(NP,1);
MT=rand(NT,1)*sum(MP)*0.5;
%% end of random data simulation

% проверка корректности размерности матриц
NP=size(costPT,1);
NT=size(costPT,2);
NU=size(costPU,2);
if NP~=size(costPU,1) || NP~=numel(MP)
    error('? incorrect sizes NT??')
end
if NT~=numel(costTU) || NT~=numel(MT)
    error('? incorrect sizes NT??')
end

%"карты" переменных икс
% переменные икс по сути матрицы, но в задаче ЛП нужно их вытянуть в один
% столбец.... так напишем же в ячеку матриц соотв.размера номер
% соответсвующего места в столбце
mapXpt=nan(size(costPT));
mapXpu=nan(size(costPU));
co=0;
for p=1:NP
    for t=1:NT
        co=co+1;
        mapXpt(p,t)=co;
    end
end
for p=1:NP
    for u=1:NU
        co=co+1;
        mapXpu(p,u)=co;
    end
end
NX=co;%число переменных

%формирование матриц
%Каждая помойка куда-то привязана
AeqUnikPrivyaz=sparse(NP,NX);
for p=1:NP
    for t=1:NT
        AeqUnikPrivyaz(p,mapXpt(p,t))=1;
    end
    for u=1:NU
        AeqUnikPrivyaz(p,mapXpu(p,u))=1;
    end
end
beqUnikPrivyaz=ones(NP,1);

%Мощность каждой перевалки не превышена
AineqMoschnost=sparse(NT,NX);
for t=1:NT
    for p=1:NP
        AineqMoschnost(t,mapXpt(p,t))=MP(p);
    end
end
bineqMoschnost=MT;

%целевая функция
flp=ones(NX,1);
for p=1:NP
    for t=1:NT
        flp(mapXpt(p,t))=(costPT(p,t)+costTU(t))*MP(p);
    end
end
for p=1:NP
    for u=1:NU
        flp(mapXpu(p,u))=costPT(p,u)*MP(p);
    end
end



%% %%%%%%%%%%%%%%%%%%%%%%%%%%%%%
x = intlinprog(flp,1:NX,AineqMoschnost,bineqMoschnost,AeqUnikPrivyaz,beqUnikPrivyaz, zeros(size(flp)), ones(size(flp)));
%% %%%%%%%%%%%%%%%%%%%%%%%%%%%%%


%% декодирование решения в привычную матричную форму
%% Xpu - матрица оптимальных иксов кав в ворд-файле x(p,u)
%% Xpt - матрица оптимальных иксов кав в ворд-файле x(p,t)
intX=round(x); % после интлинпрога и целочисленные переменные иногда не совсем целочисленны
Xpu=zeros(NP,NU);
Xpt=zeros(NP,NT);
for p=1:NP
    for t=1:NT
        Xpt(p,t)=intX(mapXpt(p,t));
    end
end
for p=1:NP
    for u=1:NU
        Xpu(p,u)=intX(mapXpu(p,u));
    end
end




