"""Reuse the first proposal's drawing primitives and shared workspace."""
from pathlib import Path
import runpy
import argparse

parser = argparse.ArgumentParser()
parser.add_argument('--revision', type=int, choices=(2, 3), default=2)
revision = parser.parse_args().revision

OUT = Path(__file__).parent
base = runpy.run_path(str(OUT / 'render_mockup.py'))
parts = base['parts']
original = list(parts)
rect, text, line, dot, icon, host = (base[k] for k in ('rect', 'text', 'line', 'dot', 'icon', 'host'))
start = next(i for i,s in enumerate(original) if '<rect x="33" y="127"' in s)
end = next(i for i,s in enumerate(original) if '<rect x="291" y="127"' in s)

def tabs(active):
    rect(33,127,257,857,'#F5F7FA')
    line(290,127,290,984)
    if revision == 3:
        text(54,153,'服务器状态',12,'#3974DC' if active=='status' else '#8792A4',600 if active=='status' else 400)
        text(144,153,'连接列表',12,'#3974DC' if active=='connections' else '#8792A4',600 if active=='connections' else 400)
        line(33,167,290,167)
        rect(54 if active=='status' else 144,165,60 if active=='status' else 48,2,'#3974DC')
        return
    rect(45,138,233,35,'#E9EDF4',6)
    rect(48 if active=='status' else 164,141,111,29,'#FFFFFF',5,'#DDE4EF')
    text(65,160,'服务器状态',12,'#3974DC' if active=='status' else '#8592A5',600 if active=='status' else 400)
    text(187,160,'连接列表',12,'#3974DC' if active=='connections' else '#8592A5',600 if active=='connections' else 400)

def meter(y,label,value,ratio,detail):
    text(54,y,label,12,'#7A899E')
    text(213,y,value,15,'#415571',600)
    rect(54,y+13,214,5,'#E5EBF4',2)
    rect(54,y+13,214*ratio,5,'#7299DC',2)
    text(54,y+38,detail,11,'#93A0B2')

def status():
    tabs('status')
    icon(54,194,'server','#668ABD')
    text(89,205,'生产应用 01',15,'#354D6D',600)
    text(89,226,'root@10.0.1.21',11,'#8A98AA')
    dot(59,252,3,'#55B38C');text(69,256,'已连接',11,'#67927C')
    text(152,256,'更新于 14:32:08',10,'#98A3B3')
    line(53,275,268,275)
    meter(305,'CPU 使用率' if revision == 3 else '处理器','8.2%',.082,'负载  0.12 / 0.18 / 0.15')
    meter(386,'内存使用率' if revision == 3 else '内存','42.5%',.425,'已用 6.8 / 共 16.0 GiB' if revision == 3 else '6.8 / 16.0 GiB')
    line(53,443,268,443)
    text(54,472,'磁盘',13,'#536781',600)
    text(54,504,'/data',12,'#74859E',mono=True);text(233,504,'64%',12,'#536781',600)
    rect(54,516,214,5,'#E5EBF4',2);rect(54,516,137,5,'#7299DC',2)
    text(54,541,'128 / 200 GiB',11,'#93A0B2')
    text(54,573,'/',12,'#74859E',mono=True);text(233,573,'28%',12,'#536781',600)
    rect(54,585,214,5,'#E5EBF4',2);rect(54,585,60,5,'#7299DC',2)
    text(54,610,'11.2 / 40 GiB',11,'#93A0B2')
    line(53,631,268,631)
    text(54,660,'网络',13,'#536781',600);text(235,660,'eth0',11,'#93A0B2')
    text(54,691,'↓ 接收',11,'#8898AC');text(170,691,'128.4 KB/s',12,'#536781')
    text(54,721,'↑ 发送',11,'#8898AC');text(178,721,'32.1 KB/s',12,'#536781')
    line(53,745,268,745)
    text(54,776,'系统信息',13,'#536781',600)
    text(54,808,'系统',11,'#8898AC');text(154,808,'Ubuntu 24.04',11,'#536781')
    text(54,837,'运行时间',11,'#8898AC');text(193,837,'12 天 3 时',11,'#536781')
    line(53,862,268,862)
    text(54,893,'进程',12,'#74859E');text(250,893,'›',18,'#96A3B6')
    line(53,909,268,909,'#EAF0F6')
    text(54,940,'端口连接',12,'#74859E');text(250,940,'›',18,'#96A3B6')

def connections():
    tabs('connections')
    rect(51,190,221,36,'#FFFFFF',6,'#E1E6EE')
    icon(62,197,'search','#9AA4B4');text(92,213,'搜索连接',13,'#9AA4B4')
    icon(53,244,'chevron');text(81,261,'生产环境',12,'#7F8A9B',600);text(246,261,'3',12,'#929CAD')
    host(284,'生产应用 01','10.0.1.21',True,True)
    host(354,'生产应用 02','10.0.1.22')
    host(424,'生产数据库','10.0.1.30')
    icon(53,507,'chevron');text(81,524,'测试环境',12,'#7F8A9B',600);text(246,524,'2',12,'#929CAD')
    host(548,'测试服务器','10.0.2.10',live=True)
    host(618,'开发服务器','10.0.2.11')
    rect(51,876,221,39,'#FFFFFF',6,'#DCE3ED');icon(103,884,'plus','#5878AA');text(132,901,'新建连接',13,'#4B6385',500)
    text(136,944,'导入连接',12,'#8894A6')

for name, draw, label in [('status',status,'02  服务器状态'),('connections',connections,'02  连接列表')]:
    parts.clear()
    parts.extend(original[:start])
    draw()
    parts.extend(original[end:])
    svg='\n'.join(parts).replace('界面改版 · 方案 01',f'界面改版 · 方案 {revision:02d}').replace('01  主工作台',label.replace('02 ',f'{revision:02d} '))
    (OUT/f'nexshell-workspace-v{revision}-{name}.svg').write_text(svg)
