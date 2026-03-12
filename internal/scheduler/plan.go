package scheduler

import "time"

func InitialNextRun(def Definition, now time.Time) (time.Time, error) {
	if def.RunImmediately {
		return now, nil
	}
	return NextRunAfter(def, now)
}

func NextRunAfter(def Definition, after time.Time) (time.Time, error) {
	if def.Every != "" {
		interval, err := parsePositiveDuration(def.Every)
		if err != nil {
			return time.Time{}, err
		}
		return after.Add(interval), nil
	}

	cronSchedule, err := ParseCron(def.Cron)
	if err != nil {
		return time.Time{}, err
	}
	return cronSchedule.NextAfter(after)
}
