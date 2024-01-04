import { ValueOf } from "../util";
import { JsonApi } from "./jsonApi";

const areaCodeMap = {
    "宗谷地方": "011000",
    "上川・留萌地方": "012000",
    "石狩・空知・後志地方": "016000",
    "網走・北見・紋別地方": "013000",
    "釧路・根室地方": "014100",
    "胆振・日高地方": "015000",
    "渡島・檜山地方": "017000",
    "青森県": "020000",
    "秋田県": "050000",
    "岩手県": "030000",
    "宮城県": "040000",
    "山形県": "060000",
    "福島県": "070000",
    "茨城県": "080000",
    "栃木県": "090000",
    "群馬県": "100000",
    "埼玉県": "110000",
    "東京都": "130000",
    "千葉県": "120000",
    "神奈川県": "140000",
    "長野県": "200000",
    "山梨県": "190000",
    "静岡県": "220000",
    "愛知県": "230000",
    "岐阜県": "210000",
    "三重県": "240000",
    "新潟県": "150000",
    "富山県": "160000",
    "石川県": "170000",
    "福井県": "180000",
    "滋賀県": "250000",
    "京都府": "260000",
    "大阪府": "270000",
    "兵庫県": "280000",
    "奈良県": "290000",
    "和歌山県": "300000",
    "岡山県": "330000",
    "広島県": "340000",
    "島根県": "320000",
    "鳥取県": "310000",
    "徳島県": "360000",
    "香川県": "370000",
    "愛媛県": "380000",
    "高知県": "390000",
    "山口県": "350000",
    "福岡県": "400000",
    "大分県": "440000",
    "長崎県": "420000",
    "佐賀県": "410000",
    "熊本県": "430000",
    "宮崎県": "450000",
    "鹿児島県": "460100",
    "沖縄本島地方": "471000",
    "大東島地方": "472000",
    "宮古島地方": "473000",
    "八重山地方": "474000"
} as const;

type AreaCode = ValueOf<typeof areaCodeMap>;

interface RawTimeSeriesItem {
    timeDefines: string[];
    areas: {
        area: { name: string, code: AreaCode },
        weathers: string[],
        winds: string[],
        waves?: string[],
        temps?: number[],
    }[];
}

interface RawWeatherForecast {
    publishingOffice: string;
    reportDateTime: string;
    timeSeries: RawTimeSeriesItem[];
}

export interface AreaForecast {
    areaName: string;
    areaCode: AreaCode;
    weathers: {
        time: string;
        weather?: string;
        wind?: string;
        wave?: string;
    }[];
}

export interface TempertureForecast {
    areaName: string;
    tempertures?: {
        time: string;
        temperture?: number;
    }[];
}

export interface WeatherForecast {
    reportDateTime: string;
    areaForecasts: AreaForecast[];
    tempertureForecasts: TempertureForecast[];
}

export class JmaApi {
    private readonly jsonApi: JsonApi;

    constructor() {
        this.jsonApi = new JsonApi('https://www.jma.go.jp/bosai/forecast/data');
    }

    getAreaCodeMap(): Record<string, AreaCode> {
        return areaCodeMap;
    }

    async getWeatherForecast(code: AreaCode): Promise<WeatherForecast> {
        const rawForecasts = await this.jsonApi.get<RawWeatherForecast[]>(`/forecast/${code}.json`);
        // rawForecasts[0] = 天気予報
        // rawForecasts[1] = ?
        const rawForecast = rawForecasts[0];
        const threeDaySeries = rawForecast.timeSeries[0];
        const tempertureSeries = rawForecast.timeSeries[2];
        const areaForecasts = threeDaySeries.areas.map((a) => ({
            areaName: a.area.name,
            areaCode: a.area.code,
            weathers: threeDaySeries.timeDefines.map((t, j) => ({
                time: t,
                weather: a.weathers && a.weathers[j],
                wind: a.winds && a.winds[j],
                wave: a.waves && a.waves[j],
            })),
        } satisfies AreaForecast));
        const tempertureForecasts = tempertureSeries.areas.map((a) => ({
            areaName: a.area.name,
            tempertures: a.temps?.map((t, i) => ({
                time: tempertureSeries.timeDefines[i],
                temperture: t,
            })),
        } satisfies TempertureForecast))
        return {
            reportDateTime: rawForecast.reportDateTime,
            areaForecasts,
            tempertureForecasts,
        };
    }
}